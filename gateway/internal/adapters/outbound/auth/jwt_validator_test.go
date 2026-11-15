/**
##
## OverDrive 2026
## All Technical rights reserved
##
## jwt_validator_test.go - Unit tests for JWTValidator.
##
*/

package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidator_ValidateBearerHeader(t *testing.T) {
	secret := "test-secret"
	validator := NewJWTValidator(secret)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "user-123",
		"email": "user@example.com",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(15 * time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString returned error: %v", err)
	}

	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{"valid jwt bearer header", "Bearer " + signed, true},
		{"case-insensitive scheme", "bearer " + signed, true},
		{"valid token with extra whitespace", "Bearer   " + signed, true},
		{"wrong scheme", "Basic " + signed, false},
		{"missing token", "Bearer ", false},
		{"empty header", "", false},
		{"tampered token", "Bearer " + signed + "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validator.ValidateBearerHeader(tt.header); got != tt.want {
				t.Errorf("ValidateBearerHeader(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}
