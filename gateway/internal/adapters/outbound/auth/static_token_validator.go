// Package auth contains outbound auth adapter implementations.

package auth

import "strings"

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

	return token == v.expectedToken
}
