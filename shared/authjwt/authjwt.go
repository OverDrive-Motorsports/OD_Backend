/**
##
## OverDrive 2026
## All Technical rights reserved
##
## authjwt.go - Shared verifier for the access tokens issued by auth-service.
##
*/

// Package authjwt verifies the HS256 access tokens issued by auth-service. It is the single
// implementation used by every component that authenticates end users (auth-service itself,
// the gateway and user-data-service), so a fix to this security-critical path lands everywhere.
package authjwt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidToken covers every rejection except expiry: malformed token, bad signature,
	// unexpected algorithm, missing or empty `sub` claim.
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired is returned for an otherwise valid token past its `exp` claim, so callers
	// can tell the client to refresh rather than to log in again.
	ErrTokenExpired = errors.New("token expired")
	// ErrMissingToken is returned by VerifyBearerHeader when the header is absent, uses
	// another scheme than Bearer, or carries no token.
	ErrMissingToken = errors.New("missing bearer token")
	// ErrMissingSecret is returned by NewVerifier when no signing secret is configured.
	ErrMissingSecret = errors.New("jwt secret is required")
	// ErrWeakSecret is returned by NewVerifier for a secret shorter than MinSecretBytes or equal
	// to a known placeholder value.
	ErrWeakSecret = errors.New("jwt secret is too weak")
)

// MinSecretBytes is the minimum length of AUTH_JWT_SECRET: an HS256 key should carry at least
// 256 bits (RFC 7518 §3.2), i.e. 32 bytes.
const MinSecretBytes = 32

// placeholderSecrets are values shipped in templates or examples; a deployment still using one
// is misconfigured even if the value is long enough.
var placeholderSecrets = map[string]struct{}{
	"change-me-in-production": {},
}

// ValidateSecret reports whether secret can sign or verify access tokens: non-blank, at least
// MinSecretBytes long and not a known placeholder. Every component holding AUTH_JWT_SECRET goes
// through it (via NewVerifier), so a weak secret stops the process at startup.
func ValidateSecret(secret string) error {
	if strings.TrimSpace(secret) == "" {
		return ErrMissingSecret
	}
	if _, placeholder := placeholderSecrets[secret]; placeholder || len(secret) < MinSecretBytes {
		return fmt.Errorf("%w: at least %d bytes required, and not a template placeholder", ErrWeakSecret, MinSecretBytes)
	}
	return nil
}

// Verifier checks tokens against the shared AUTH_JWT_SECRET.
type Verifier struct {
	secret []byte
}

// NewVerifier builds a verifier for the given HS256 secret. A missing or weak secret is refused
// (see ValidateSecret) so a misconfigured process fails at startup instead of accepting tokens
// signed with a guessable key.
func NewVerifier(secret string) (*Verifier, error) {
	if err := ValidateSecret(secret); err != nil {
		return nil, err
	}
	return &Verifier{secret: []byte(secret)}, nil
}

// Verify validates the token signature (HS256 only — any other algorithm, including `none`,
// is rejected), its expiry, and returns the authenticated user ID from the `sub` claim.
func (v *Verifier) Verify(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(*jwt.Token) (any, error) {
		return v.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrTokenExpired
		}
		return "", ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", ErrInvalidToken
	}
	userID, ok := claims["sub"].(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return "", ErrInvalidToken
	}
	return userID, nil
}

// BearerToken extracts the token from an `Authorization: Bearer <token>` header value
// (scheme matched case-insensitively). ok is false when the header is empty, uses another
// scheme, or carries no token.
func BearerToken(header string) (token string, ok bool) {
	scheme, value, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, value != ""
}

// VerifyBearerHeader extracts the token from an `Authorization` header value and verifies it.
// The error is ErrMissingToken, ErrTokenExpired or ErrInvalidToken, so every caller (gateway,
// services) answers the same 401 code for the same situation.
func (v *Verifier) VerifyBearerHeader(header string) (string, error) {
	token, ok := BearerToken(header)
	if !ok {
		return "", ErrMissingToken
	}
	return v.Verify(token)
}
