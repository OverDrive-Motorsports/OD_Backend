/**
##
## OverDrive 2026
## All Technical rights reserved
##
## token_validator.go - Declares token validation port used by authorization logic.
##
*/

// Package ports defines gateway core ports.

package ports

import "overdrive/gateway/internal/core/domain"

// TokenValidator checks the static gateway token (internal and scripted calls).
type TokenValidator interface {
	ValidateBearerHeader(header string) bool
}

// UserTokenValidator verifies an end-user access token. It returns the token's role when it is
// valid, or domain.ErrMissingUserToken / ErrUserTokenExpired / ErrInvalidUserToken.
type UserTokenValidator interface {
	ValidateUserBearerHeader(header string) (domain.UserRole, error)
}
