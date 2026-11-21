/**
##
## OverDrive 2026
## All Technical rights reserved
##
## user_token_validator.go - Validates end-user access tokens (JWT issued by auth-service).
##
*/

package auth

import (
	"errors"

	"overdrive/gateway/internal/core/domain"
	"overdrive/shared/authjwt"
)

// UserTokenValidator accepts an `Authorization: Bearer <jwt>` header carrying a valid access
// token issued by auth-service. It guards every route that needs a user (data, presets,
// sessions) and the admin routes; services that need the caller's identity re-verify the
// token and read the user ID from it.
type UserTokenValidator struct {
	verifier *authjwt.Verifier
}

// NewUserTokenValidator builds a validator for the shared AUTH_JWT_SECRET.
func NewUserTokenValidator(secret string) (UserTokenValidator, error) {
	verifier, err := authjwt.NewVerifier(secret)
	if err != nil {
		return UserTokenValidator{}, err
	}
	return UserTokenValidator{verifier: verifier}, nil
}

// ValidateUserBearerHeader returns the role of a valid, unexpired user JWT, otherwise the domain
// reason: missing/non-Bearer header, expired token, or any other invalid token.
func (v UserTokenValidator) ValidateUserBearerHeader(header string) (domain.UserRole, error) {
	if v.verifier == nil {
		return "", domain.ErrInvalidUserToken
	}
	claims, err := v.verifier.VerifyBearerHeaderClaims(header)
	switch {
	case err == nil:
		return domain.UserRole(claims.Role), nil
	case errors.Is(err, authjwt.ErrMissingToken):
		return "", domain.ErrMissingUserToken
	case errors.Is(err, authjwt.ErrTokenExpired):
		return "", domain.ErrUserTokenExpired
	default:
		return "", domain.ErrInvalidUserToken
	}
}
