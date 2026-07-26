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

type SessionQueryUseCase interface {
	GetUserSessions(ctx context.Context, userID string) ([]domain.AuthSession, error)
	GetUserSession(ctx context.Context, userID string, sessionID string) (*domain.AuthSession, error)
	GetSessionByID(ctx context.Context, sessionID string) (*domain.AuthSession, error)
	AddUserSession(ctx context.Context, userID string) (*domain.AuthSession, error)
	RemoveUserSession(ctx context.Context, userID string, sessionID string) error
}
