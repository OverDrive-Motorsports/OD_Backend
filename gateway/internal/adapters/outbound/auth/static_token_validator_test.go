/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## static_token_validator_test.go - Unit tests for StaticTokenValidator.
 ##
 */

package auth

import "testing"

func TestStaticTokenValidator_ValidateBearerHeader(t *testing.T) {
	validator := NewStaticTokenValidator("expected-token")

	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{"valid bearer header", "Bearer expected-token", true},
		{"case-insensitive scheme", "bearer expected-token", true},
		{"wrong token", "Bearer wrong-token", false},
		{"empty header", "", false},
		{"whitespace only header", "   ", false},
		{"missing scheme", "expected-token", false},
		{"missing token", "Bearer", false},
		{"empty token after scheme", "Bearer ", false},
		{"wrong scheme", "Basic expected-token", false},
		{"extra whitespace around token still trims to match", "Bearer  expected-token", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validator.ValidateBearerHeader(tt.header); got != tt.want {
				t.Errorf("ValidateBearerHeader(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}
