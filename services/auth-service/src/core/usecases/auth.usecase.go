/**
##
## OverDrive 2026
## All Technical rights reserved
##
## auth.usecase.go - Package usecases source file for services/auth-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"overdrive/services/auth-service/src/core/domain"
	"overdrive/services/auth-service/src/core/ports"
)

type AuthQueryUseCase struct {
	repository ports.SessionRepository
}

// NewAuthQueryUseCase builds and returns a catalog query use case with its required dependencies.
func NewAuthQueryUseCase(repository ports.SessionRepository) *AuthQueryUseCase {
	return &AuthQueryUseCase{repository: repository}
}

func (u *AuthQueryUseCase) Login(ctx context.Context, email string, password string) (*domain.AuthSession, error) {
	println("email: "+email, "password: "+password)
	return u.repository.AddUserSession(ctx, "test")
}

func (u *AuthQueryUseCase) Register(ctx context.Context, email string, password string, username string) (*domain.AuthSession, error) {
	println("email: "+email, "password: "+password, "username: "+username)
	return u.repository.AddUserSession(ctx, "test")
}
