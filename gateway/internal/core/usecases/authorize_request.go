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
