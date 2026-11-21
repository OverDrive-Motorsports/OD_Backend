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

import (
	"overdrive/gateway/internal/core/domain"
	"overdrive/gateway/internal/core/ports"
)

// AuthorizeUserRequestUseCase authorises a request carrying an end-user access token of any
// role and tells why it is rejected (see ports.UserTokenValidator).
type AuthorizeUserRequestUseCase struct {
	validator ports.UserTokenValidator
}

func NewAuthorizeUserRequestUseCase(validator ports.UserTokenValidator) AuthorizeUserRequestUseCase {
	return AuthorizeUserRequestUseCase{validator: validator}
}

// Execute returns nil when the header carries a valid user token, the rejection reason otherwise.
func (u AuthorizeUserRequestUseCase) Execute(authHeader string) error {
	if u.validator == nil {
		return domain.ErrInvalidUserToken
	}
	_, err := u.validator.ValidateUserBearerHeader(authHeader)
	return err
}

// AuthorizeAdminRequestUseCase authorises the administration routes: an access token with the
// admin role, or the static gateway token kept for internal and scripted calls.
type AuthorizeAdminRequestUseCase struct {
	gatewayToken ports.TokenValidator
	userToken    ports.UserTokenValidator
}

func NewAuthorizeAdminRequestUseCase(gatewayToken ports.TokenValidator, userToken ports.UserTokenValidator) AuthorizeAdminRequestUseCase {
	return AuthorizeAdminRequestUseCase{gatewayToken: gatewayToken, userToken: userToken}
}

// Execute returns nil for the gateway token or an admin token, domain.ErrForbiddenRole for a
// valid token of another role, and the user-token rejection reason otherwise.
func (u AuthorizeAdminRequestUseCase) Execute(authHeader string) error {
	if u.gatewayToken != nil && u.gatewayToken.ValidateBearerHeader(authHeader) {
		return nil
	}
	if u.userToken == nil {
		return domain.ErrInvalidUserToken
	}
	role, err := u.userToken.ValidateUserBearerHeader(authHeader)
	if err != nil {
		return err
	}
	if role != domain.RoleAdmin {
		return domain.ErrForbiddenRole
	}
	return nil
}
