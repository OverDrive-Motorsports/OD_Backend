/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session.usecase.go - Package usecases source file for services/auth-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"overdrive/services/auth-service/src/core/domain"
	"overdrive/services/auth-service/src/core/ports"
)

type SessionQueryUseCase struct {
	repository ports.SessionRepository
}

// NewSessionQueryUseCase builds and returns a catalog query use case with its required dependencies.
func NewSessionQueryUseCase(repository ports.SessionRepository) *SessionQueryUseCase {
	return &SessionQueryUseCase{repository: repository}
}

func (u *SessionQueryUseCase) GetUserSessions(ctx context.Context, userID string) ([]domain.AuthSession, error) {
	return u.repository.GetUserSessions(ctx, userID)
}

func (u *SessionQueryUseCase) GetUserSession(ctx context.Context, userID string, sessionID string) (*domain.AuthSession, error) {
	return u.repository.GetUserSession(ctx, userID, sessionID)
}

func (u *SessionQueryUseCase) GetSessionByID(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	return u.repository.GetSessionByID(ctx, sessionID)
}

func (u *SessionQueryUseCase) AddUserSession(ctx context.Context, userID string) (*domain.AuthSession, error) {
	return u.repository.AddUserSession(ctx, userID)
}

func (u *SessionQueryUseCase) RemoveUserSession(ctx context.Context, userID string, sessionID string) error {
	return u.repository.RemoveUserSession(ctx, userID, sessionID)
}
