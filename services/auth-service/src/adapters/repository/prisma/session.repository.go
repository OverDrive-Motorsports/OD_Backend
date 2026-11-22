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

	"golang.org/x/crypto/bcrypt"
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
		sessions = append(sessions, mapAuthSessionMin(&row))
	}
	return sessions, nil
}

func (r *SessionRepository) GetUserSession(ctx context.Context, userID string, sessionID string) (*domain.AuthSession, error) {
	row, err := r.client.AuthSession.FindFirst(
		db.AuthSession.ID.Equals(sessionID),
		db.AuthSession.UserID.Equals(userID),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	session := mapAuthSession(row, "")
	return &session, nil
}

func (r *SessionRepository) GetSessionByID(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	row, err := r.client.AuthSession.FindUnique(
		db.AuthSession.ID.Equals(sessionID),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	session := mapAuthSession(row, "")
	return &session, nil
}

func (r *SessionRepository) AddUserSession(ctx context.Context, userID string) (*domain.AuthSession, error) {
	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshTokenHash, err := hashRefreshToken(refreshToken)
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

	session := mapAuthSession(row, refreshToken)
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

func (r *SessionRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	row, err := r.client.User.FindFirst(
		db.User.ID.Equals(userID),
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

// GetUserByOAuthAccount returns the user linked to a provider identity, or nil if none is.
func (r *SessionRepository) GetUserByOAuthAccount(ctx context.Context, provider domain.OAuthProviderType, subject string) (*domain.User, error) {
	row, err := r.client.OAuthAccount.FindUnique(
		db.OAuthAccount.ProviderProviderSubject(
			db.OAuthAccount.Provider.Equals(db.ProviderType(provider)),
			db.OAuthAccount.ProviderSubject.Equals(subject),
		),
	).With(
		db.OAuthAccount.User.Fetch(),
	).Exec(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	user := mapUser(row.User())
	return &user, nil
}

// AddOAuthUser creates a password-less user and its OAuthAccount link atomically.
func (r *SessionRepository) AddOAuthUser(ctx context.Context, email string, username string, provider domain.OAuthProviderType, subject string) (*domain.User, error) {
	userID, err := generateUUID()
	if err != nil {
		return nil, err
	}

	createUser := r.client.User.CreateOne(
		db.User.Email.Set(email),
		db.User.DisplayName.Set(username),
		db.User.ID.Set(userID),
	).Tx()
	createAccount := r.client.OAuthAccount.CreateOne(
		db.OAuthAccount.Provider.Set(db.ProviderType(provider)),
		db.OAuthAccount.ProviderSubject.Set(subject),
		db.OAuthAccount.User.Link(db.User.ID.Equals(userID)),
	).Tx()

	if err := r.client.Prisma.Transaction(createUser, createAccount).Exec(ctx); err != nil {
		return nil, err
	}

	user := mapUser(createUser.Result())
	return &user, nil
}

// generateUUID returns a random RFC 4122 version 4 UUID.
func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}

func generateRefreshToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(randomBytes), nil
}

func (r *SessionRepository) RefreshToken(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshTokenHash, err := hashRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	row, err := r.client.AuthSession.FindUnique(
		db.AuthSession.ID.Equals(sessionID),
	).Update(
		db.AuthSession.RefreshTokenHash.Set(refreshTokenHash),
		db.AuthSession.ExpiresAt.Set(time.Now().Add(30*24*time.Hour)),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	session := mapAuthSession(row, refreshToken)
	return &session, nil
}

func hashRefreshToken(refreshToken string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(refreshToken), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// mapAuthSession maps db auth session rows into an internal ingestion dataset.
func mapAuthSession(row *db.AuthSessionModel, refreshToken string) domain.AuthSession {
	if refreshToken == "" {
		refreshToken = row.RefreshTokenHash
	}

	return domain.AuthSession{
		ID:           row.ID,
		UserID:       row.UserID,
		RefreshToken: refreshToken,
		ExpiresAt:    row.ExpiresAt,
		CreatedAt:    row.CreatedAt,
	}
}

// mapAuthSession maps db auth session rows into an internal ingestion dataset.
func mapAuthSessionMin(row *db.AuthSessionModel) domain.AuthSession {
	return domain.AuthSession{
		ID:        row.ID,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
	}
}

// mapUser maps db user rows into an internal user dataset.
func mapUser(row *db.UserModel) domain.User {
	var passwordHash string
	var usesOAuth = false

	// A user without a password can only have been created through an OAuth provider.
	if v, ok := row.PasswordHash(); ok {
		passwordHash = v
	} else {
		usesOAuth = true
	}

	return domain.User{
		ID:           row.ID,
		Email:        row.Email,
		DisplayName:  row.DisplayName,
		PasswordHash: passwordHash,
		UsesOAuth:    usesOAuth,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
