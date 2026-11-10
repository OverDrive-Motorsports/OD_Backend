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

	ListOAuthProviders() []domain.OAuthProviderInfo
	OAuthAuthorize(providerID string) (*domain.OAuthAuthorization, error)
	OAuthLogin(ctx context.Context, providerID string, callback domain.OAuthCallback) (*domain.LoginResponse, error)
	OAuthRegister(ctx context.Context, providerID string, callback domain.OAuthCallback, username string) (*domain.LoginResponse, error)

	Refresh(ctx context.Context, refreshToken string, sessionID string) (*domain.RefreshResponse, error)
	VerifyToken(tokenString string) (string, error)
}
