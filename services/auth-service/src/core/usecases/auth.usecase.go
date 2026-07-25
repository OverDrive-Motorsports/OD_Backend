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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"overdrive/services/auth-service/src/core/domain"
	"overdrive/services/auth-service/src/core/ports"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthQueryUseCase struct {
	repository ports.SessionRepository
}

// NewAuthQueryUseCase builds and returns a catalog query use case with its required dependencies.
func NewAuthQueryUseCase(repository ports.SessionRepository) *AuthQueryUseCase {
	return &AuthQueryUseCase{repository: repository}
}

func (u *AuthQueryUseCase) Login(ctx context.Context, email string, password string) (*domain.LoginResponse, error) {
	user, err := u.repository.GetUserByEmail(ctx, email)

	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid credentials")
	}

	if hashPassword(password) != user.PasswordHash {
		return nil, errors.New("invalid credentials")
	}
	token, err := generateJWT(user.ID, user.Email)
	session, err := u.repository.AddUserSession(ctx, user.ID)

	if err != nil {
		return nil, err
	}
	return &domain.LoginResponse{
		Token:        token,
		SessionID:    session.ID,
		RefreshToken: session.RefreshTokenHash,
		ExpiresAt:    session.ExpiresAt,
	}, nil
}

func (u *AuthQueryUseCase) Register(ctx context.Context, email string, password string, username string) (*domain.LoginResponse, error) {
	if existingUser, err := u.repository.GetUserByEmail(ctx, email); err != nil {
		return nil, err
	} else if existingUser != nil {
		return nil, errors.New("email already registered")
	}

	passwordHash := hashPassword(password)
	user, err := u.repository.AddUser(ctx, email, passwordHash, username)
	if err != nil {
		return nil, err
	}
	token, err := generateJWT(user.ID, user.Email)
	session, err := u.repository.AddUserSession(ctx, user.ID)

	if err != nil {
		return nil, err
	}
	return &domain.LoginResponse{
		Token:        token,
		SessionID:    session.ID,
		RefreshToken: session.RefreshTokenHash,
		ExpiresAt:    session.ExpiresAt,
	}, nil
}

func (u *AuthQueryUseCase) Refresh(ctx context.Context, refreshToken string, sessionID string) (*domain.RefreshResponse, error) {
	existingSession, err := u.repository.GetUserSession(ctx, sessionID)

	if err != nil {
		return nil, err
	}
	if existingSession == nil || existingSession.RefreshTokenHash != refreshToken {
		return nil, errors.New("invalid session")
	}

	session, err := u.repository.RefreshToken(ctx, sessionID)

	if err != nil {
		return nil, err
	}
	return &domain.RefreshResponse{
		RefreshToken: session.RefreshTokenHash,
		ExpiresAt:    session.ExpiresAt,
	}, nil
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func generateJWT(userID string, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(15 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := os.Getenv("AUTH_JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}

	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	_, err = jwt.Parse(signedToken, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	return signedToken, nil
}
