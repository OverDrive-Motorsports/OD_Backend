/**
##
## OverDrive 2026
## All Technical rights reserved
##
## static_token_validator.go - Outbound adapter that validates a static bearer token.
##
*/

// Package auth contains outbound auth adapter implementations.

package auth

import (
	"crypto/subtle"
	"strings"
)

type StaticTokenValidator struct {
	expectedToken string
}

func NewStaticTokenValidator(expectedToken string) StaticTokenValidator {
	return StaticTokenValidator{expectedToken: expectedToken}
}

func (v StaticTokenValidator) ValidateBearerHeader(header string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return false
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return false
	}

	// Constant-time comparison: a plain == returns as soon as a byte differs, which leaks how
	// much of the token an attacker guessed right through response timing.
	return subtle.ConstantTimeCompare([]byte(token), []byte(v.expectedToken)) == 1
}
