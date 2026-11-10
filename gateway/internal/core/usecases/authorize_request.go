/**
##
## OverDrive 2026
## All Technical rights reserved
##
## authorize_request.go - Use case validating authorization headers for requests.
##
*/

// Package usecases contains gateway application use cases.

package usecases

import "overdrive/gateway/internal/core/ports"

type AuthorizeRequestUseCase struct {
	validator ports.TokenValidator
}

func NewAuthorizeRequestUseCase(validator ports.TokenValidator) AuthorizeRequestUseCase {
	return AuthorizeRequestUseCase{validator: validator}
}

func (u AuthorizeRequestUseCase) Execute(authHeader string) bool {
	return u.validator.ValidateBearerHeader(authHeader)
}

// AuthorizeUserRequestUseCase authorises a request carrying an end-user access token and tells
// why it is rejected (see ports.UserTokenValidator).
type AuthorizeUserRequestUseCase struct {
	validator ports.UserTokenValidator
}

func NewAuthorizeUserRequestUseCase(validator ports.UserTokenValidator) AuthorizeUserRequestUseCase {
	return AuthorizeUserRequestUseCase{validator: validator}
}

// Execute returns nil when the header carries a valid user token, the rejection reason otherwise.
func (u AuthorizeUserRequestUseCase) Execute(authHeader string) error {
	return u.validator.ValidateUserBearerHeader(authHeader)
}
