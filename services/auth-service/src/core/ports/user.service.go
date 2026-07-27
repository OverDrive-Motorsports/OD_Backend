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

type UserQueryUseCase interface {
	GetUsers(ctx context.Context) ([]domain.User, error)
	GetUser(ctx context.Context, userID string) (*domain.User, error)
	CreateUser(ctx context.Context, user domain.User) (*domain.User, error)
	DeleteUser(ctx context.Context, userID string) error
}
