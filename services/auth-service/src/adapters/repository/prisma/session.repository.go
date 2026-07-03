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
	"crypto/rand"
	"encoding/hex"
	"errors"
	db "overdrive/services/auth-service/resources/db"
	"overdrive/services/auth-service/src/core/domain"
	"time"
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
	refreshTokenHash, err := generateRefreshTokenHash()
	if err != nil {
		return nil, err
	}

	row, err := r.client.AuthSession.CreateOne(
		db.AuthSession.RefreshTokenHash.Set(refreshTokenHash),
		db.AuthSession.ExpiresAt.Set(time.Now().Add(30*24*time.Hour)),
		db.AuthSession.User.Link(db.User.ID.Equals(userID)),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	session := mapAuthSession(row)
	return &session, nil
}

func (r *SessionRepository) RemoveUserSession(ctx context.Context, userID string, sessionID string) error {
	_, err := r.client.AuthSession.FindMany(
		db.AuthSession.UserID.Equals(userID),
		db.AuthSession.ID.Equals(sessionID),
	).Delete().Exec(ctx)

	return err
}

func (r *SessionRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := r.client.User.FindFirst(
		db.User.Email.Equals(email),
	).Exec(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	user := mapUser(row)
	return &user, nil
}

func (r *SessionRepository) AddUser(ctx context.Context, email string, passwordHash string, username string) (*domain.User, error) {
	row, err := r.client.User.CreateOne(
		db.User.Email.Set(email),
		db.User.DisplayName.Set(username),
		db.User.PasswordHash.Set(passwordHash),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	user := mapUser(row)
	return &user, nil
}

func generateRefreshTokenHash() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(randomBytes), nil
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

// mapUser maps db user rows into an internal user dataset.
func mapUser(row *db.UserModel) domain.User {
	return domain.User{
		ID:           row.ID,
		Email:        row.Email,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
