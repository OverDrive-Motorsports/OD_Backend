/**
##
## OverDrive 2026
## All Technical rights reserved
##
## oauth.usecase.go - Provider-agnostic OAuth sign-in / sign-up flow of AuthQueryUseCase.
##
*/

package usecases

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"overdrive/services/auth-service/src/core/domain"
	"overdrive/services/auth-service/src/core/ports"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
)

const (
	oauthStateType     = "oauth_state"
	oauthStateLifetime = 10 * time.Minute
	maxDisplayNameLen  = 50 // User.displayName is VarChar(50)
)

// ListOAuthProviders returns the enabled providers, in registration order.
func (u *AuthQueryUseCase) ListOAuthProviders() []domain.OAuthProviderInfo {
	infos := make([]domain.OAuthProviderInfo, 0, len(u.oauthProviderOrder))
	for _, id := range u.oauthProviderOrder {
		infos = append(infos, domain.OAuthProviderInfo{ID: id, Name: u.oauthProviders[id].Name()})
	}
	return infos
}

// OAuthAuthorize starts a flow: it returns the provider consent URL, a signed state bound to
// this provider, and the PKCE verifier the client must send back with the code.
func (u *AuthQueryUseCase) OAuthAuthorize(providerID string) (*domain.OAuthAuthorization, error) {
	provider, err := u.oauthProvider(providerID)
	if err != nil {
		return nil, err
	}

	state, err := u.generateOAuthState(provider.ID())
	if err != nil {
		return nil, err
	}
	verifier, err := randomURLSafe(32)
	if err != nil {
		return nil, err
	}

	return &domain.OAuthAuthorization{
		URL:          provider.AuthCodeURL(state, pkceChallenge(verifier)),
		State:        state,
		CodeVerifier: verifier,
	}, nil
}

// OAuthLogin signs in the user already linked to the provider identity behind callback.Code.
func (u *AuthQueryUseCase) OAuthLogin(ctx context.Context, providerID string, callback domain.OAuthCallback) (*domain.LoginResponse, error) {
	identity, err := u.resolveOAuthIdentity(ctx, providerID, callback)
	if err != nil {
		return nil, err
	}

	user, err := u.repository.GetUserByOAuthAccount(ctx, identity.Provider, identity.Subject)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrOAuthAccountNotLinked
	}
	return u.startSession(ctx, user)
}

// OAuthRegister creates a password-less user linked to the provider identity, then signs it in.
// username is optional; the provider's display name (or the email local part) is used otherwise.
func (u *AuthQueryUseCase) OAuthRegister(ctx context.Context, providerID string, callback domain.OAuthCallback, username string) (*domain.LoginResponse, error) {
	identity, err := u.resolveOAuthIdentity(ctx, providerID, callback)
	if err != nil {
		return nil, err
	}

	if linked, err := u.repository.GetUserByOAuthAccount(ctx, identity.Provider, identity.Subject); err != nil {
		return nil, err
	} else if linked != nil {
		return nil, domain.ErrOAuthAccountAlreadyLinked
	}

	// Linking a provider to an existing (e.g. password) account must be an explicit,
	// authenticated action; doing it silently here would let the provider take over the account.
	if existing, err := u.repository.GetUserByEmail(ctx, identity.Email); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, domain.ErrEmailAlreadyRegistered
	}

	user, err := u.repository.AddOAuthUser(ctx, identity.Email, oauthDisplayName(username, identity), identity.Provider, identity.Subject)
	if err != nil {
		return nil, err
	}
	return u.startSession(ctx, user)
}

// resolveOAuthIdentity validates the callback and exchanges its code with the provider.
func (u *AuthQueryUseCase) resolveOAuthIdentity(ctx context.Context, providerID string, callback domain.OAuthCallback) (*domain.OAuthIdentity, error) {
	provider, err := u.oauthProvider(providerID)
	if err != nil {
		return nil, err
	}
	if err := u.verifyOAuthState(callback.State, provider.ID()); err != nil {
		return nil, err
	}
	if callback.Code == "" || callback.CodeVerifier == "" {
		return nil, domain.ErrOAuthExchangeFailed
	}

	identity, err := provider.ExchangeCode(ctx, callback.Code, callback.CodeVerifier)
	if err != nil {
		return nil, err
	}
	if identity.Email == "" || !identity.EmailVerified {
		return nil, domain.ErrOAuthEmailNotVerified
	}
	identity.Provider = provider.Type()
	identity.Email = strings.ToLower(strings.TrimSpace(identity.Email))
	return identity, nil
}

func (u *AuthQueryUseCase) oauthProvider(providerID string) (ports.OAuthProvider, error) {
	provider, ok := u.oauthProviders[providerID]
	if !ok {
		return nil, domain.ErrUnknownOAuthProvider
	}
	return provider, nil
}

// generateOAuthState returns a short-lived JWT bound to providerID. It is stateless (no DB
// row): the signature proves this service issued it, "typ" keeps it from being confused with
// an access token (which VerifyToken also rejects as it has no "sub").
func (u *AuthQueryUseCase) generateOAuthState(providerID string) (string, error) {
	nonce, err := randomURLSafe(16)
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"typ":   oauthStateType,
		"prv":   providerID,
		"nonce": nonce,
		"iat":   now.Unix(),
		"exp":   now.Add(oauthStateLifetime).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(u.jwtSecret))
}

func (u *AuthQueryUseCase) verifyOAuthState(state string, providerID string) error {
	if state == "" {
		return domain.ErrInvalidOAuthState
	}
	token, err := jwt.Parse(state, func(token *jwt.Token) (any, error) {
		return []byte(u.jwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return domain.ErrInvalidOAuthState
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["typ"] != oauthStateType || claims["prv"] != providerID {
		return domain.ErrInvalidOAuthState
	}
	return nil
}

func oauthDisplayName(requested string, identity *domain.OAuthIdentity) string {
	name := strings.TrimSpace(requested)
	if name == "" {
		name = strings.TrimSpace(identity.DisplayName)
	}
	if name == "" {
		name, _, _ = strings.Cut(identity.Email, "@")
	}
	for utf8.RuneCountInString(name) > maxDisplayNameLen {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomURLSafe(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
