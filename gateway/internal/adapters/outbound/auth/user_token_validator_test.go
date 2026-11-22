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

// TestNewUserTokenValidator_RequiresSecret proves the gateway cannot be built without
// AUTH_JWT_SECRET.
func TestNewUserTokenValidator_RequiresSecret(t *testing.T) {
	if _, err := NewUserTokenValidator(""); err == nil {
		t.Fatal("expected an error for an empty secret")
	}
}

// TestUserTokenValidator_ValidateUserBearerHeader proves each rejection maps to the domain
// reason the gateway turns into a distinct 401 code — and that the static gateway token is not
// a valid user token.
func TestUserTokenValidator_ValidateUserBearerHeader(t *testing.T) {
	validator, err := NewUserTokenValidator("secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := validator.ValidateUserBearerHeader("Bearer " + signUserToken(t, "secret", time.Now().Add(time.Minute))); err != nil {
		t.Fatalf("expected a valid user JWT to be accepted, got %v", err)
	}
	cases := map[string]struct {
		header string
		want   error
	}{
		"empty":        {"", domain.ErrMissingUserToken},
		"no scheme":    {signUserToken(t, "secret", time.Now().Add(time.Minute)), domain.ErrMissingUserToken},
		"expired":      {"Bearer " + signUserToken(t, "secret", time.Now().Add(-time.Minute)), domain.ErrUserTokenExpired},
		"wrong secret": {"Bearer " + signUserToken(t, "other", time.Now().Add(time.Minute)), domain.ErrInvalidUserToken},
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
