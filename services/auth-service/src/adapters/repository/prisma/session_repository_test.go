/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## session_repository_test.go - Package prismaadapter source file for services/auth-service/src/adapters/repository/prisma.
	##
*/

package prismaadapter

import (
	"testing"
	"time"

	db "overdrive/services/auth-service/resources/db"

	"golang.org/x/crypto/bcrypt"
)

// TestGenerateRefreshToken_NonEmptyAndDistinct sanity-checks generateRefreshToken produces a
// non-empty, non-predictable value - not a full statistical randomness proof, just confirming
// repeated calls don't collide/return a constant across a reasonable sample size.
func TestGenerateRefreshToken_NonEmptyAndDistinct(t *testing.T) {
	seen := make(map[string]bool)
	const samples = 100

	for i := 0; i < samples; i++ {
		token, err := generateRefreshToken()
		if err != nil {
			t.Fatalf("generateRefreshToken failed: %v", err)
		}
		if token == "" {
			t.Fatalf("expected a non-empty token")
		}
		if seen[token] {
			t.Fatalf("expected distinct tokens across %d samples, got a collision: %s", samples, token)
		}
		seen[token] = true
	}
}

// TestHashRefreshToken_RoundTrip proves hashRefreshToken produces a hash distinct from the
// input plaintext, and that bcrypt.CompareHashAndPassword against the original token succeeds.
func TestHashRefreshToken_RoundTrip(t *testing.T) {
	hash, err := hashRefreshToken("my-refresh-token")
	if err != nil {
		t.Fatalf("hashRefreshToken failed: %v", err)
	}
	if hash == "my-refresh-token" {
		t.Fatalf("expected hash to differ from plaintext input")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("my-refresh-token")); err != nil {
		t.Fatalf("expected the generated hash to validate against the original refresh token: %v", err)
	}
}

// TestMapAuthSession_UsesSuppliedRefreshTokenWhenPresent proves mapAuthSession prefers the
// explicitly supplied plaintext refresh token (the one just generated, before it was hashed for
// storage) over the row's stored hash.
func TestMapAuthSession_UsesSuppliedRefreshTokenWhenPresent(t *testing.T) {
	expiresAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	row := &db.AuthSessionModel{
		InnerAuthSession: db.InnerAuthSession{
			ID:               "session-1",
			UserID:           "user-1",
			RefreshTokenHash: "stored-hash",
			ExpiresAt:        expiresAt,
			CreatedAt:        createdAt,
		},
	}

	session := mapAuthSession(row, "plaintext-refresh-token")

	if session.ID != "session-1" || session.UserID != "user-1" {
		t.Fatalf("unexpected identity fields: %+v", session)
	}
	if session.RefreshToken != "plaintext-refresh-token" {
		t.Fatalf("expected the supplied plaintext refresh token to be used, got %q", session.RefreshToken)
	}
	if !session.ExpiresAt.Equal(expiresAt) || !session.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected timestamps to be copied verbatim, got %+v", session)
	}
}

// TestMapAuthSession_FallsBackToStoredHashWhenRefreshTokenEmpty proves mapAuthSession falls
// back to the row's stored hash when no plaintext refresh token is supplied (the read-path
// case, where the plaintext token was never persisted and only its hash exists).
func TestMapAuthSession_FallsBackToStoredHashWhenRefreshTokenEmpty(t *testing.T) {
	row := &db.AuthSessionModel{
		InnerAuthSession: db.InnerAuthSession{
			ID:               "session-1",
			RefreshTokenHash: "stored-hash",
		},
	}

	session := mapAuthSession(row, "")

	if session.RefreshToken != "stored-hash" {
		t.Fatalf("expected fallback to the stored hash, got %q", session.RefreshToken)
	}
}

// TestMapAuthSessionMin_OmitsUserIDAndRefreshToken proves mapAuthSessionMin only copies the
// identity/timestamp fields it's documented to (used by GetUserSessions, a listing endpoint
// that intentionally doesn't expose refresh tokens).
func TestMapAuthSessionMin_OmitsUserIDAndRefreshToken(t *testing.T) {
	expiresAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	row := &db.AuthSessionModel{
		InnerAuthSession: db.InnerAuthSession{
			ID:               "session-1",
			UserID:           "user-1",
			RefreshTokenHash: "stored-hash",
			ExpiresAt:        expiresAt,
			CreatedAt:        createdAt,
		},
	}

	session := mapAuthSessionMin(row)

	if session.ID != "session-1" {
		t.Fatalf("expected ID to be copied, got %q", session.ID)
	}
	if session.UserID != "" {
		t.Fatalf("expected UserID to be omitted, got %q", session.UserID)
	}
	if session.RefreshToken != "" {
		t.Fatalf("expected RefreshToken to be omitted, got %q", session.RefreshToken)
	}
	if !session.ExpiresAt.Equal(expiresAt) || !session.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected timestamps to still be copied, got %+v", session)
	}
}

// TestMapUser_CopiesAllFields proves mapUser copies every field verbatim from the Prisma-
// generated row into the domain type.
func TestMapUser_CopiesAllFields(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	row := &db.UserModel{
		InnerUser: db.InnerUser{
			ID:           "user-1",
			Email:        "user@example.com",
			DisplayName:  "User One",
			PasswordHash: "hashed-password",
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
		},
	}

	user := mapUser(row)

	if user.ID != "user-1" || user.Email != "user@example.com" || user.DisplayName != "User One" || user.PasswordHash != "hashed-password" {
		t.Fatalf("unexpected mapped user: %+v", user)
	}
	if !user.CreatedAt.Equal(createdAt) || !user.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected timestamps to be copied verbatim, got %+v", user)
	}
}
