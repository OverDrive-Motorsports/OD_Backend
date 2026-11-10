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
// token issued by auth-service. It guards the per-user routes (/presets), where the client
// sends its own JWT instead of the shared gateway token; the upstream service re-verifies the
// token and reads the user ID from it.
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

// ValidateUserBearerHeader returns nil for a valid, unexpired user JWT, otherwise the domain
// reason: missing/non-Bearer header, expired token, or any other invalid token.
func (v UserTokenValidator) ValidateUserBearerHeader(header string) error {
	if v.verifier == nil {
		return domain.ErrInvalidUserToken
	}
	_, err := v.verifier.VerifyBearerHeader(header)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, authjwt.ErrMissingToken):
		return domain.ErrMissingUserToken
	case errors.Is(err, authjwt.ErrTokenExpired):
		return domain.ErrUserTokenExpired
	default:
		return domain.ErrInvalidUserToken
	}
}
