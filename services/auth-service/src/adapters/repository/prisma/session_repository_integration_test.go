/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_repository_integration_test.go - Integration tests of the auth repository against Postgres.
##
*/

package prismaadapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"

	db "overdrive/services/auth-service/resources/db"
	"overdrive/services/auth-service/src/core/domain"
	"overdrive/services/auth-service/src/core/usecases"
)

// integrationJWTSecret signs the access tokens of the end-to-end refresh test (32 bytes or more).
const integrationJWTSecret = "integration-secret-0123456789abcdef-0123456789"

// integrationClient connects to the database named by INTEGRATION_DATABASE_URL, or skips the
// test when it is unset (plain `go test`). The database name must end with "_test": the tests
// write rows, so a development database is refused, and the database actually reached is
// checked after connecting.
func integrationClient(t *testing.T) *db.PrismaClient {
	t.Helper()
	raw := os.Getenv("INTEGRATION_DATABASE_URL")
	if raw == "" {
		t.Skip("INTEGRATION_DATABASE_URL is not set: run tools/integration-test.sh")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !strings.HasSuffix(strings.TrimPrefix(parsed.Path, "/"), "_test") {
		t.Fatal("INTEGRATION_DATABASE_URL must name a database ending with _test")
	}
	// The schema takes its URL from env(AUTH_DATABASE_URL), which the query engine reads from the
	// environment itself: db.WithDatasourceURL is ignored for such a schema.
	t.Setenv("AUTH_DATABASE_URL", raw)
	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Prisma.Disconnect(); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	})
	var current []struct {
		Name string `json:"name"`
	}
	if err := client.Prisma.QueryRaw(`SELECT current_database()::text AS name`).Exec(context.Background(), &current); err != nil ||
		len(current) != 1 || !strings.HasSuffix(current[0].Name, "_test") {
		t.Fatalf("connected to %v (err %v), want a database ending with _test", current, err)
	}
	return client
}

// uniqueSuffix returns a random hex string, so tests share the database without cleaning it.
func uniqueSuffix(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b[:])
}

// addPasswordUser stores a password user and returns it with its plain password.
func addPasswordUser(t *testing.T, repo *SessionRepository) (*domain.User, string) {
	t.Helper()
	const password = "integration-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	user, err := repo.AddUser(context.Background(), "it-"+uniqueSuffix(t)+"@example.com", string(hash), "it-user")
	if err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	return user, password
}

// TestIntegrationRefreshToken_RotatesStoredHash proves a refresh replaces the stored hash: the
// new token matches it, the old one no longer does, and the expiry moves forward. Only a hash
// is stored, never the token.
func TestIntegrationRefreshToken_RotatesStoredHash(t *testing.T) {
	repo := NewSessionRepository(integrationClient(t))
	ctx := context.Background()
	user, _ := addPasswordUser(t, repo)

	created, err := repo.AddUserSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("AddUserSession: %v", err)
	}
	stored, err := repo.GetSessionByID(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("GetSessionByID: %v %v", stored, err)
	}
	if stored.RefreshToken == created.RefreshToken {
		t.Fatal("the refresh token is stored in clear")
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.RefreshToken), []byte(created.RefreshToken)) != nil {
		t.Fatal("the stored hash does not match the issued token")
	}

	refreshed, err := repo.RefreshToken(ctx, created.ID)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if refreshed.RefreshToken == created.RefreshToken || refreshed.UserID != user.ID {
		t.Fatalf("refreshed = %+v, want a new token for the same user", refreshed)
	}
	stored, err = repo.GetSessionByID(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("GetSessionByID after refresh: %v %v", stored, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.RefreshToken), []byte(refreshed.RefreshToken)) != nil {
		t.Fatal("the stored hash does not match the new token")
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.RefreshToken), []byte(created.RefreshToken)) == nil {
		t.Fatal("the old token still matches the stored hash")
	}
	if stored.ExpiresAt.Before(created.ExpiresAt) {
		t.Fatalf("expiry moved back: %v < %v", stored.ExpiresAt, created.ExpiresAt)
	}
}

// TestIntegrationRefresh_OldTokenRefusedAfterRotation proves, through the real use case on
// the real repository, that a refresh token works once: after a refresh the previous token is
// refused (ErrInvalidToken) and the new one is accepted.
func TestIntegrationRefresh_OldTokenRefusedAfterRotation(t *testing.T) {
	repo := NewSessionRepository(integrationClient(t))
	ctx := context.Background()
	user, password := addPasswordUser(t, repo)
	uc := usecases.NewAuthQueryUseCase(repo, integrationJWTSecret)

	login, err := uc.Login(ctx, user.Email, password)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	first, err := uc.Refresh(ctx, login.RefreshToken, login.SessionID)
	if err != nil {
		t.Fatalf("first Refresh: %v", err)
	}
	if _, err := uc.Refresh(ctx, login.RefreshToken, login.SessionID); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("replayed token: err = %v, want ErrInvalidToken", err)
	}
	if _, err := uc.Refresh(ctx, first.RefreshToken, login.SessionID); err != nil {
		t.Fatalf("Refresh with the rotated token: %v", err)
	}
}

// TestIntegrationRemoveUserSession_Revocation proves revocation: another user cannot revoke a
// session (ErrSessionNotFound, session kept), the owner can, the revoked session is gone from
// every read and can no longer be refreshed, and a second revocation reports not found.
func TestIntegrationRemoveUserSession_Revocation(t *testing.T) {
	repo := NewSessionRepository(integrationClient(t))
	ctx := context.Background()
	owner, password := addPasswordUser(t, repo)
	other, _ := addPasswordUser(t, repo)
	uc := usecases.NewAuthQueryUseCase(repo, integrationJWTSecret)

	login, err := uc.Login(ctx, owner.Email, password)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	kept, err := repo.AddUserSession(ctx, owner.ID)
	if err != nil {
		t.Fatalf("AddUserSession: %v", err)
	}

	if err := repo.RemoveUserSession(ctx, other.ID, login.SessionID); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("revocation by another user: err = %v, want ErrSessionNotFound", err)
	}
	if session, err := repo.GetUserSession(ctx, owner.ID, login.SessionID); err != nil || session == nil {
		t.Fatalf("session after a foreign revocation = %v, %v; want it kept", session, err)
	}

	if err := repo.RemoveUserSession(ctx, owner.ID, login.SessionID); err != nil {
		t.Fatalf("revocation by the owner: %v", err)
	}
	if session, err := repo.GetUserSession(ctx, owner.ID, login.SessionID); err != nil || session != nil {
		t.Fatalf("revoked session = %v, %v; want nil, nil", session, err)
	}
	sessions, err := repo.GetUserSessions(ctx, owner.ID)
	if err != nil || len(sessions) != 1 || sessions[0].ID != kept.ID {
		t.Fatalf("remaining sessions = %+v (err %v), want only %s", sessions, err, kept.ID)
	}
	if _, err := uc.Refresh(ctx, login.RefreshToken, login.SessionID); !errors.Is(err, domain.ErrInvalidSession) {
		t.Fatalf("refresh of a revoked session: err = %v, want ErrInvalidSession", err)
	}
	if err := repo.RemoveUserSession(ctx, owner.ID, login.SessionID); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("second revocation: err = %v, want ErrSessionNotFound", err)
	}
}

// TestIntegrationAddOAuthUser_CreatesUserAndLink proves an OAuth sign-up stores a
// password-less user and its provider link, found back by the provider identity and by email.
func TestIntegrationAddOAuthUser_CreatesUserAndLink(t *testing.T) {
	repo := NewSessionRepository(integrationClient(t))
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email, subject := "oauth-"+suffix+"@example.com", "subject-"+suffix

	user, err := repo.AddOAuthUser(ctx, email, "oauth-user", domain.OAuthProviderGoogle, subject)
	if err != nil {
		t.Fatalf("AddOAuthUser: %v", err)
	}
	if !uuidV4Pattern.MatchString(user.ID) || !user.UsesOAuth || user.PasswordHash != "" || user.Role != domain.RoleUser {
		t.Fatalf("user = %+v, want a v4 ID, no password, role user", user)
	}
	linked, err := repo.GetUserByOAuthAccount(ctx, domain.OAuthProviderGoogle, subject)
	if err != nil || linked == nil || linked.ID != user.ID {
		t.Fatalf("GetUserByOAuthAccount = %v, %v; want %s", linked, err, user.ID)
	}
	if other, err := repo.GetUserByOAuthAccount(ctx, domain.OAuthProviderF1TV, subject); err != nil || other != nil {
		t.Fatalf("same subject on another provider = %v, %v; want nil, nil", other, err)
	}
	byEmail, err := repo.GetUserByEmail(ctx, email)
	if err != nil || byEmail == nil || byEmail.ID != user.ID || !byEmail.UsesOAuth {
		t.Fatalf("GetUserByEmail = %+v, %v", byEmail, err)
	}
}

// TestIntegrationAddOAuthUser_IsAtomic proves the user and its link are written in one
// transaction: when the link insert fails (identity already linked), the user inserted just
// before it is rolled back. A second sign-up with a taken email fails too.
func TestIntegrationAddOAuthUser_IsAtomic(t *testing.T) {
	repo := NewSessionRepository(integrationClient(t))
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email, subject := "oauth-"+suffix+"@example.com", "subject-"+suffix
	if _, err := repo.AddOAuthUser(ctx, email, "first", domain.OAuthProviderGoogle, subject); err != nil {
		t.Fatalf("AddOAuthUser: %v", err)
	}

	freshEmail := "fresh-" + suffix + "@example.com"
	if _, err := repo.AddOAuthUser(ctx, freshEmail, "second", domain.OAuthProviderGoogle, subject); err == nil {
		t.Fatal("linking an already linked identity must fail")
	}
	if user, err := repo.GetUserByEmail(ctx, freshEmail); err != nil || user != nil {
		t.Fatalf("user left behind by the failed link = %v, %v; want nil, nil", user, err)
	}

	if _, err := repo.AddOAuthUser(ctx, email, "third", domain.OAuthProviderGoogle, "fresh-subject-"+suffix); err == nil {
		t.Fatal("a second user with the same email must fail")
	}
}

// TestIntegrationAddUser_ConcurrentRegistration proves the email unique constraint holds when
// the same email registers several times at once: exactly one insert succeeds and every other
// one gets the database's unique-constraint error on the email (the use case's existence check
// runs before the insert, so this error is what a request losing the race receives).
func TestIntegrationAddUser_ConcurrentRegistration(t *testing.T) {
	repo := NewSessionRepository(integrationClient(t))
	ctx := context.Background()
	email := "race-" + uniqueSuffix(t) + "@example.com"
	const attempts = 8

	var wg sync.WaitGroup
	results := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.AddUser(ctx, email, "hash", "racer")
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
			continue
		}
		violation, ok := db.IsErrUniqueConstraint(err)
		if !ok || len(violation.Fields) != 1 || violation.Fields[0] != db.User.Email.Field() {
			t.Fatalf("losing registration: err = %v, want a unique-constraint error on the email", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("%d registrations succeeded, want exactly 1", succeeded)
	}
	if user, err := repo.GetUserByEmail(ctx, email); err != nil || user == nil {
		t.Fatalf("GetUserByEmail = %v, %v", user, err)
	}
}
