/**
##
## OverDrive 2026
## All Technical rights reserved
##
## user_token_validator_test.go - Unit tests for the end-user JWT validator.
##
*/

package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"overdrive/gateway/internal/core/domain"
)

func signUserToken(t *testing.T, secret string, exp time.Time) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-1", "exp": exp.Unix()}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return token
}

// validatorSecret is long enough for authjwt.ValidateSecret (32 bytes minimum).
const validatorSecret = "validator-secret-long-enough-for-hs256"

// TestNewUserTokenValidator_RequiresSecret proves the gateway cannot be built without
// AUTH_JWT_SECRET, nor with a weak one (shorter than 32 bytes).
func TestNewUserTokenValidator_RequiresSecret(t *testing.T) {
	for _, secret := range []string{"", "dev-local-jwt-secret"} {
		if _, err := NewUserTokenValidator(secret); err == nil {
			t.Fatalf("expected an error for secret %q", secret)
		}
	}
}

// TestUserTokenValidator_ValidateUserBearerHeader proves each rejection maps to the domain
// reason the gateway turns into a distinct 401 code — and that the static gateway token is not
// a valid user token.
func TestUserTokenValidator_ValidateUserBearerHeader(t *testing.T) {
	validator, err := NewUserTokenValidator(validatorSecret)
	if err != nil {
		t.Fatal(err)
	}
	if err := validator.ValidateUserBearerHeader("Bearer " + signUserToken(t, validatorSecret, time.Now().Add(time.Minute))); err != nil {
		t.Fatalf("expected a valid user JWT to be accepted, got %v", err)
	}
	cases := map[string]struct {
		header string
		want   error
	}{
		"empty":        {"", domain.ErrMissingUserToken},
		"no scheme":    {signUserToken(t, validatorSecret, time.Now().Add(time.Minute)), domain.ErrMissingUserToken},
		"expired":      {"Bearer " + signUserToken(t, validatorSecret, time.Now().Add(-time.Minute)), domain.ErrUserTokenExpired},
		"wrong secret": {"Bearer " + signUserToken(t, "another-secret-long-enough-for-hs256-keys", time.Now().Add(time.Minute)), domain.ErrInvalidUserToken},
		"static token": {"Bearer gateway-static-token", domain.ErrInvalidUserToken},
	}
	for name, tc := range cases {
		if err := validator.ValidateUserBearerHeader(tc.header); !errors.Is(err, tc.want) {
			t.Fatalf("%s: expected %v, got %v", name, tc.want, err)
		}
	}
	if err := (UserTokenValidator{}).ValidateUserBearerHeader("Bearer x"); !errors.Is(err, domain.ErrInvalidUserToken) {
		t.Fatalf("zero validator: expected ErrInvalidUserToken, got %v", err)
	}
}
