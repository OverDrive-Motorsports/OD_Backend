/**
##
## OverDrive 2026
## All Technical rights reserved
##
## oauth_usecase_test.go - Tests for the OAuth flow of AuthQueryUseCase.
##
*/

package usecases

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// fakeOAuthProvider is a ports.OAuthProvider returning a canned identity.
type fakeOAuthProvider struct {
	id       string
	identity *domain.OAuthIdentity
	err      error

	gotCode     string
	gotVerifier string
}

func (p *fakeOAuthProvider) ID() string                     { return p.id }
func (p *fakeOAuthProvider) Name() string                   { return strings.ToUpper(p.id) }
func (p *fakeOAuthProvider) Type() domain.OAuthProviderType { return domain.OAuthProviderGoogle }

func (p *fakeOAuthProvider) AuthCodeURL(state string, codeChallenge string) string {
	return "https://provider.test/auth?" + url.Values{"state": {state}, "code_challenge": {codeChallenge}}.Encode()
}

func (p *fakeOAuthProvider) ExchangeCode(ctx context.Context, code string, codeVerifier string) (*domain.OAuthIdentity, error) {
	p.gotCode, p.gotVerifier = code, codeVerifier
	if p.err != nil {
		return nil, p.err
	}
	identity := *p.identity
	return &identity, nil
}

func newGoogleFake() *fakeOAuthProvider {
	return &fakeOAuthProvider{id: "google", identity: &domain.OAuthIdentity{
		Subject: "google-sub-1", Email: "User@Example.com", EmailVerified: true, DisplayName: "Jane Doe",
	}}
}

// authorize runs OAuthAuthorize and returns a callback the client would send back.
func authorize(t *testing.T, uc *AuthQueryUseCase, providerID string) domain.OAuthCallback {
	t.Helper()
	auth, err := uc.OAuthAuthorize(providerID)
	if err != nil {
		t.Fatalf("OAuthAuthorize failed: %v", err)
	}
	return domain.OAuthCallback{Code: "auth-code", State: auth.State, CodeVerifier: auth.CodeVerifier}
}

func TestListOAuthProviders_KeepsRegistrationOrder(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", &fakeOAuthProvider{id: "google"}, &fakeOAuthProvider{id: "f1tv"})

	got := uc.ListOAuthProviders()
	if len(got) != 2 || got[0].ID != "google" || got[1].ID != "f1tv" || got[0].Name != "GOOGLE" {
		t.Fatalf("unexpected providers: %+v", got)
	}
}

func TestOAuthAuthorize_UnknownProvider(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", newGoogleFake())

	if _, err := uc.OAuthAuthorize("github"); !errors.Is(err, domain.ErrUnknownOAuthProvider) {
		t.Fatalf("expected ErrUnknownOAuthProvider, got %v", err)
	}
}

// TestOAuthAuthorize_URLCarriesStateAndS256Challenge proves the consent URL embeds the returned
// state and the S256 challenge of the returned verifier (never the verifier itself).
func TestOAuthAuthorize_URLCarriesStateAndS256Challenge(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", newGoogleFake())

	auth, err := uc.OAuthAuthorize("google")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, _ := url.Parse(auth.URL)
	query := parsed.Query()
	if query.Get("state") != auth.State {
		t.Fatalf("state mismatch in URL")
	}
	if query.Get("code_challenge") != pkceChallenge(auth.CodeVerifier) || strings.Contains(auth.URL, auth.CodeVerifier) {
		t.Fatalf("URL must carry the S256 challenge, not the verifier: %s", auth.URL)
	}
}

func TestOAuthLogin_Success(t *testing.T) {
	provider := newGoogleFake()
	repo := &fakeSessionRepository{usersByOAuth: map[string]*domain.User{
		"GOOGLE:google-sub-1": {ID: "user-1", Email: "user@example.com"},
	}}
	uc := NewAuthQueryUseCase(repo, "s", provider)
	callback := authorize(t, uc, "google")

	resp, err := uc.OAuthLogin(context.Background(), "google", callback)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Token == "" || resp.SessionID != "session-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if provider.gotCode != "auth-code" || provider.gotVerifier != callback.CodeVerifier {
		t.Fatalf("provider did not receive code/verifier: %q %q", provider.gotCode, provider.gotVerifier)
	}
	if userID, err := uc.VerifyToken(resp.Token); err != nil || userID != "user-1" {
		t.Fatalf("issued token does not authenticate user-1: %q %v", userID, err)
	}
}

func TestOAuthLogin_NotLinked(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", newGoogleFake())

	_, err := uc.OAuthLogin(context.Background(), "google", authorize(t, uc, "google"))
	if !errors.Is(err, domain.ErrOAuthAccountNotLinked) {
		t.Fatalf("expected ErrOAuthAccountNotLinked, got %v", err)
	}
}

func TestOAuthLogin_RejectsInvalidStates(t *testing.T) {
	google, other := newGoogleFake(), &fakeOAuthProvider{id: "other", identity: newGoogleFake().identity}
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", google, other)
	foreignSecret := NewAuthQueryUseCase(&fakeSessionRepository{}, "other-secret", google)
	accessToken, _ := uc.generateJWT("user-1", "user@example.com")

	cases := map[string]string{
		"empty":                "",
		"garbage":              "not-a-jwt",
		"other provider state": authorize(t, uc, "other").State,
		"forged signature":     authorize(t, foreignSecret, "google").State,
		"access token":         accessToken,
	}
	for name, state := range cases {
		t.Run(name, func(t *testing.T) {
			callback := domain.OAuthCallback{Code: "auth-code", State: state, CodeVerifier: "v"}
			if _, err := uc.OAuthLogin(context.Background(), "google", callback); !errors.Is(err, domain.ErrInvalidOAuthState) {
				t.Fatalf("expected ErrInvalidOAuthState, got %v", err)
			}
		})
	}
	if google.gotCode != "" {
		t.Fatalf("provider must not be called with an invalid state")
	}
}

// TestOAuthState_IsNotAnAccessToken proves a state token can never authenticate API calls.
func TestOAuthState_IsNotAnAccessToken(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", newGoogleFake())

	if _, err := uc.VerifyToken(authorize(t, uc, "google").State); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("expected state to be rejected by VerifyToken, got %v", err)
	}
}

func TestOAuthLogin_MissingCodeOrVerifier(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", newGoogleFake())
	callback := authorize(t, uc, "google")
	callback.CodeVerifier = ""

	if _, err := uc.OAuthLogin(context.Background(), "google", callback); !errors.Is(err, domain.ErrOAuthExchangeFailed) {
		t.Fatalf("expected ErrOAuthExchangeFailed, got %v", err)
	}
}

func TestOAuthLogin_ProviderError(t *testing.T) {
	provider := newGoogleFake()
	provider.err = domain.ErrOAuthExchangeFailed
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", provider)

	if _, err := uc.OAuthLogin(context.Background(), "google", authorize(t, uc, "google")); !errors.Is(err, domain.ErrOAuthExchangeFailed) {
		t.Fatalf("expected ErrOAuthExchangeFailed, got %v", err)
	}
}

func TestOAuthLogin_UnverifiedEmail(t *testing.T) {
	provider := newGoogleFake()
	provider.identity.EmailVerified = false
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, "s", provider)

	if _, err := uc.OAuthLogin(context.Background(), "google", authorize(t, uc, "google")); !errors.Is(err, domain.ErrOAuthEmailNotVerified) {
		t.Fatalf("expected ErrOAuthEmailNotVerified, got %v", err)
	}
}

func TestOAuthRegister_Success(t *testing.T) {
	repo := &fakeSessionRepository{}
	uc := NewAuthQueryUseCase(repo, "s", newGoogleFake())

	resp, err := uc.OAuthRegister(context.Background(), "google", authorize(t, uc, "google"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SessionID != "session-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if repo.addedOAuthLink != "GOOGLE:google-sub-1" {
		t.Fatalf("unexpected link: %q", repo.addedOAuthLink)
	}
	if repo.addedOAuthUser.Email != "user@example.com" || repo.addedOAuthUser.DisplayName != "Jane Doe" {
		t.Fatalf("expected normalized email and provider display name, got %+v", repo.addedOAuthUser)
	}
}

func TestOAuthRegister_UsesRequestedUsername(t *testing.T) {
	repo := &fakeSessionRepository{}
	uc := NewAuthQueryUseCase(repo, "s", newGoogleFake())

	if _, err := uc.OAuthRegister(context.Background(), "google", authorize(t, uc, "google"), "  speedy  "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.addedOAuthUser.DisplayName != "speedy" {
		t.Fatalf("expected requested username, got %q", repo.addedOAuthUser.DisplayName)
	}
}

func TestOAuthRegister_AlreadyLinked(t *testing.T) {
	repo := &fakeSessionRepository{usersByOAuth: map[string]*domain.User{"GOOGLE:google-sub-1": {ID: "user-1"}}}
	uc := NewAuthQueryUseCase(repo, "s", newGoogleFake())

	_, err := uc.OAuthRegister(context.Background(), "google", authorize(t, uc, "google"), "")
	if !errors.Is(err, domain.ErrOAuthAccountAlreadyLinked) || repo.addedOAuthUser != nil {
		t.Fatalf("expected ErrOAuthAccountAlreadyLinked without creating a user, got %v", err)
	}
}

// TestOAuthRegister_EmailTakenIsNotSilentlyLinked proves an existing account with the same
// email is never taken over by an OAuth sign-up.
func TestOAuthRegister_EmailTakenIsNotSilentlyLinked(t *testing.T) {
	repo := &fakeSessionRepository{usersByEmail: map[string]*domain.User{"user@example.com": {ID: "user-1"}}}
	uc := NewAuthQueryUseCase(repo, "s", newGoogleFake())

	_, err := uc.OAuthRegister(context.Background(), "google", authorize(t, uc, "google"), "")
	if !errors.Is(err, domain.ErrEmailAlreadyRegistered) || repo.addedOAuthUser != nil {
		t.Fatalf("expected ErrEmailAlreadyRegistered without creating a user, got %v", err)
	}
}

func TestOAuthDisplayName_FallbacksAndTruncation(t *testing.T) {
	if got := oauthDisplayName("", &domain.OAuthIdentity{Email: "max@example.com"}); got != "max" {
		t.Fatalf("expected email local part, got %q", got)
	}
	long := strings.Repeat("é", 60)
	if got := oauthDisplayName(long, &domain.OAuthIdentity{}); len([]rune(got)) != maxDisplayNameLen {
		t.Fatalf("expected %d runes, got %d", maxDisplayNameLen, len([]rune(got)))
	}
}

func TestNewAuthQueryUseCase_DuplicateProviderPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on duplicate provider id")
		}
	}()
	NewAuthQueryUseCase(&fakeSessionRepository{}, "s", newGoogleFake(), newGoogleFake())
}
