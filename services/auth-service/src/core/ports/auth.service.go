/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session.service.go - Package ports source file for services/auth-service/src/core/ports.
##
*/

package ports

import (
	"context"
	"overdrive/services/auth-service/src/core/domain"
)

type AuthQueryUseCase interface {
	Login(ctx context.Context, email string, password string) (*domain.AuthSession, error)
	Register(ctx context.Context, email string, password string, userName string) (*domain.AuthSession, error)
}
