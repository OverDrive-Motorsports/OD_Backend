/**
##
## OverDrive 2026
## All Technical rights reserved
##
## authjwt_test.go - Unit tests for the shared access-token verifier.
##
*/

package authjwt

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return token
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(time.Minute).Unix()}
}

// TestNewVerifier_RequiresSecret proves an empty or blank secret is refused at construction.
func TestNewVerifier_RequiresSecret(t *testing.T) {
	for _, secret := range []string{"", "   "} {
		if _, err := NewVerifier(secret); !errors.Is(err, ErrMissingSecret) {
			t.Fatalf("secret %q: expected ErrMissingSecret, got %v", secret, err)
		}
	}
}

// TestVerifier_Verify proves a valid HS256 token yields its `sub`, an expired one is
// ErrTokenExpired, and every other defect (wrong secret, `none` algorithm, other HMAC size,
// missing / empty / non-string `sub`, missing `exp`, garbage) is ErrInvalidToken.
func TestVerifier_Verify(t *testing.T) {
	verifier, err := NewVerifier(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	key := []byte(testSecret)

	userID, err := verifier.Verify(sign(t, jwt.SigningMethodHS256, key, validClaims()))
	if err != nil || userID != "user-1" {
		t.Fatalf("valid token: got %q, %v", userID, err)
	}

	expired := jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(-time.Minute).Unix()}
	if _, err := verifier.Verify(sign(t, jwt.SigningMethodHS256, key, expired)); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired token: expected ErrTokenExpired, got %v", err)
	}

	noneToken := sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims())
	invalid := map[string]string{
		"wrong secret":   sign(t, jwt.SigningMethodHS256, []byte("other"), validClaims()),
		"alg none":       noneToken,
		"HS512":          sign(t, jwt.SigningMethodHS512, key, validClaims()),
		"missing sub":    sign(t, jwt.SigningMethodHS256, key, jwt.MapClaims{"exp": time.Now().Add(time.Minute).Unix()}),
		"empty sub":      sign(t, jwt.SigningMethodHS256, key, jwt.MapClaims{"sub": " ", "exp": time.Now().Add(time.Minute).Unix()}),
		"non-string sub": sign(t, jwt.SigningMethodHS256, key, jwt.MapClaims{"sub": 42, "exp": time.Now().Add(time.Minute).Unix()}),
		"missing exp":    sign(t, jwt.SigningMethodHS256, key, jwt.MapClaims{"sub": "user-1"}),
		"not a jwt":      "not-a-jwt",
		"empty":          "",
	}
	for name, token := range invalid {
		if _, err := verifier.Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("%s: expected ErrInvalidToken, got %v", name, err)
		}
	}
}

// TestBearerToken proves the scheme is matched case-insensitively, surrounding whitespace is
// ignored, and an empty header, another scheme or a missing token is rejected.
func TestBearerToken(t *testing.T) {
	for header, want := range map[string]string{"Bearer abc": "abc", "bearer abc": "abc", "  Bearer   abc  ": "abc"} {
		if got, ok := BearerToken(header); !ok || got != want {
			t.Fatalf("%q: got %q ok=%v", header, got, ok)
		}
	}
	for _, header := range []string{"", "Bearer", "Bearer   ", "Basic abc", "abc"} {
		if got, ok := BearerToken(header); ok {
			t.Fatalf("%q: expected rejection, got %q", header, got)
		}
	}
}

// TestVerifier_VerifyBearerHeader proves the three outcomes callers map to distinct 401 codes:
// ErrMissingToken (no / non-Bearer header), ErrTokenExpired, ErrInvalidToken — and the user ID
// on success.
func TestVerifier_VerifyBearerHeader(t *testing.T) {
	verifier, _ := NewVerifier(testSecret)
	key := []byte(testSecret)

	if userID, err := verifier.VerifyBearerHeader("Bearer " + sign(t, jwt.SigningMethodHS256, key, validClaims())); err != nil || userID != "user-1" {
		t.Fatalf("valid header: got %q, %v", userID, err)
	}
	for _, header := range []string{"", "Basic abc", "Bearer "} {
		if _, err := verifier.VerifyBearerHeader(header); !errors.Is(err, ErrMissingToken) {
			t.Fatalf("%q: expected ErrMissingToken, got %v", header, err)
		}
	}
	expired := jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(-time.Minute).Unix()}
	if _, err := verifier.VerifyBearerHeader("Bearer " + sign(t, jwt.SigningMethodHS256, key, expired)); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired: expected ErrTokenExpired, got %v", err)
	}
	if _, err := verifier.VerifyBearerHeader("Bearer forged"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("forged: expected ErrInvalidToken, got %v", err)
	}
}
