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
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// TestUserTokenValidator_ValidateBearerHeader proves only a Bearer header carrying a valid,
// unexpired JWT signed with the shared secret is accepted, the static gateway token is not.
func TestUserTokenValidator_ValidateBearerHeader(t *testing.T) {
	validator, err := NewUserTokenValidator("secret")
	if err != nil {
		t.Fatal(err)
	}
	if !validator.ValidateBearerHeader("Bearer " + signUserToken(t, "secret", time.Now().Add(time.Minute))) {
		t.Fatal("expected a valid user JWT to be accepted")
	}
	for name, header := range map[string]string{
		"expired":       "Bearer " + signUserToken(t, "secret", time.Now().Add(-time.Minute)),
		"wrong secret":  "Bearer " + signUserToken(t, "other", time.Now().Add(time.Minute)),
		"static token":  "Bearer gateway-static-token",
		"no scheme":     signUserToken(t, "secret", time.Now().Add(time.Minute)),
		"empty":         "",
		"zero verifier": "Bearer x",
	} {
		v := validator
		if name == "zero verifier" {
			v = UserTokenValidator{}
		}
		if v.ValidateBearerHeader(header) {
			t.Fatalf("%s: expected rejection", name)
		}
	}
}
