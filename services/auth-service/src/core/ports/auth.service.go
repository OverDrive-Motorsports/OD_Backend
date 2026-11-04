/**
##
## OverDrive 2026
## All Technical rights reserved
##
## auth.service.go - Package ports source file for services/auth-service/src/core/ports.
##
*/

package ports

import (
	"context"
	"overdrive/services/auth-service/src/core/domain"
)

type AuthQueryUseCase interface {
	Login(ctx context.Context, email string, password string) (*domain.LoginResponse, error)
	Register(ctx context.Context, email string, password string, userName string) (*domain.LoginResponse, error)

	OAuthLogin(ctx context.Context) (*domain.LoginResponse, error)    //TODO: Add func params and update return LoginResponse fields
	OAuthRegister(ctx context.Context) (*domain.LoginResponse, error) //TODO: Add func params and update return LoginResponse fields

	Refresh(ctx context.Context, refreshToken string, sessionID string) (*domain.RefreshResponse, error)
	VerifyToken(tokenString string) (string, error)
}
