/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session.service.go - Package ports source file for services/auth-service/src/core/ports.
##
*/

package prismaadapter

import (
	"context"
	db "overdrive/services/auth-service/resources/db"
	"overdrive/services/auth-service/src/core/domain"
)

type SessionRepository struct {
	client *db.PrismaClient
}

// NewSessionRepository builds and returns a session repository with its required dependencies.
func NewSessionRepository(client *db.PrismaClient) *SessionRepository {
	return &SessionRepository{client: client}
}

func (r *SessionRepository) GetUserSessions(ctx context.Context, userID string) ([]domain.AuthSession, error) {
	rows, err := r.client.AuthSession.FindMany(
		db.AuthSession.UserID.Equals(userID),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	sessions := make([]domain.AuthSession, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, mapAuthSession(&row))
	}
	return sessions, nil
}

func (r *SessionRepository) GetUserSession(ctx context.Context, userID string, sessionID string) (*domain.AuthSession, error) {
	row, err := r.client.AuthSession.FindFirst(
		db.AuthSession.UserID.Equals(userID),
		db.AuthSession.ID.Equals(sessionID),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	session := mapAuthSession(row)
	return &session, nil
}

func (r *SessionRepository) AddUserSession(ctx context.Context, userID string) (*domain.AuthSession, error) {
	return nil, nil //TODO
}

func (r *SessionRepository) RemoveUserSession(ctx context.Context, userID string, sessionID string) error {
	return nil //TODO
}

// mapAuthSession maps db auth session rows into an internal ingestion dataset.
func mapAuthSession(row *db.AuthSessionModel) domain.AuthSession {
	return domain.AuthSession{
		ID:               row.ID,
		UserID:           row.UserID,
		RefreshTokenHash: row.RefreshTokenHash,
		ExpiresAt:        row.ExpiresAt,
		CreatedAt:        row.CreatedAt,
	}
}
