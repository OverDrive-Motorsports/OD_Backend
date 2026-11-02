/**
##
## OverDrive 2026
## All Technical rights reserved
##
## user_token_validator.go - Validates end-user access tokens (JWT issued by auth-service).
##
*/

package auth

import "overdrive/shared/authjwt"

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

// ValidateBearerHeader reports whether the header carries a valid, unexpired user JWT.
func (v UserTokenValidator) ValidateBearerHeader(header string) bool {
	token, ok := authjwt.BearerToken(header)
	if !ok || v.verifier == nil {
		return false
	}
	_, err := v.verifier.Verify(token)
	return err == nil
}
