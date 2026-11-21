/**
##
## OverDrive 2026
## All Technical rights reserved
##
## auth_usecase_test.go - Package usecases source file for services/auth-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
	"overdrive/shared/authjwt"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// testJWTSecret is long enough for authjwt.ValidateSecret (32 bytes minimum), which
// VerifyToken goes through.
const testJWTSecret = "test-secret-long-enough-for-hs256-keys"

// fakeSessionRepository is an in-memory ports.SessionRepository used to exercise
// AuthQueryUseCase without a DB. Each method's behavior is driven by the struct's fields so
// individual tests can configure only what they need.
type fakeSessionRepository struct {
	usersByEmail map[string]*domain.User
	usersByID    map[string]*domain.User
	sessionsByID map[string]*domain.AuthSession

	addUserErr        error
	addUserSessionErr error
	getUserByEmailErr error
	getSessionErr     error
	refreshTokenErr   error
	getUserByIDErr    error

	addedUser    *domain.User
	refreshCalls int

	// usersByOAuth is keyed by provider + ":" + subject.
	usersByOAuth    map[string]*domain.User
	addOAuthUserErr error
	addedOAuthUser  *domain.User
	addedOAuthLink  string
}

func (f *fakeSessionRepository) GetUserByOAuthAccount(ctx context.Context, provider domain.OAuthProviderType, subject string) (*domain.User, error) {
	return f.usersByOAuth[string(provider)+":"+subject], nil
}

func (f *fakeSessionRepository) AddOAuthUser(ctx context.Context, email string, username string, provider domain.OAuthProviderType, subject string) (*domain.User, error) {
	if f.addOAuthUserErr != nil {
		return nil, f.addOAuthUserErr
	}
	f.addedOAuthUser = &domain.User{ID: "oauth-user", Email: email, DisplayName: username, UsesOAuth: true}
	f.addedOAuthLink = string(provider) + ":" + subject
	return f.addedOAuthUser, nil
}

func (f *fakeSessionRepository) GetUserSessions(ctx context.Context, userID string) ([]domain.AuthSession, error) {
	panic("not used by AuthQueryUseCase")
}

func (f *fakeSessionRepository) GetUserSession(ctx context.Context, userID string, sessionID string) (*domain.AuthSession, error) {
	panic("not used by AuthQueryUseCase")
}

func (f *fakeSessionRepository) GetSessionByID(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	if f.getSessionErr != nil {
		return nil, f.getSessionErr
	}
	if f.sessionsByID == nil {
		return nil, nil
	}
	session, ok := f.sessionsByID[sessionID]
	if !ok {
		return nil, nil
	}
	return session, nil
}

func (f *fakeSessionRepository) AddUserSession(ctx context.Context, userID string) (*domain.AuthSession, error) {
	if f.addUserSessionErr != nil {
		return nil, f.addUserSessionErr
	}
	return &domain.AuthSession{ID: "session-1", UserID: userID, RefreshToken: "refresh-1"}, nil
}

func (f *fakeSessionRepository) RemoveUserSession(ctx context.Context, userID string, sessionID string) error {
	panic("not used by AuthQueryUseCase")
}

func (f *fakeSessionRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	if f.getUserByEmailErr != nil {
		return nil, f.getUserByEmailErr
	}
	if f.usersByEmail == nil {
		return nil, nil
	}
	user, ok := f.usersByEmail[email]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (f *fakeSessionRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	if f.getUserByIDErr != nil {
		return nil, f.getUserByIDErr
	}
	if f.usersByID == nil {
		return nil, nil
	}
	user, ok := f.usersByID[userID]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (f *fakeSessionRepository) AddUser(ctx context.Context, email string, passwordHash string, username string) (*domain.User, error) {
	if f.addUserErr != nil {
		return nil, f.addUserErr
	}
	f.addedUser = &domain.User{ID: "new-user", Email: email, PasswordHash: passwordHash, DisplayName: username}
	return f.addedUser, nil
}

func (f *fakeSessionRepository) RefreshToken(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	f.refreshCalls++
	if f.refreshTokenErr != nil {
		return nil, f.refreshTokenErr
	}
	return &domain.AuthSession{ID: sessionID, UserID: "user-1", RefreshToken: "new-refresh-token"}, nil
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword failed: %v", err)
	}
	return string(hash)
}

// TestLogin_Success proves a matching email+password combination returns a populated
// LoginResponse including a session created via AddUserSession.
func TestLogin_Success(t *testing.T) {
	hash := mustHash(t, "correct-password")
	repo := &fakeSessionRepository{
		usersByEmail: map[string]*domain.User{
			"user@example.com": {ID: "user-1", Email: "user@example.com", PasswordHash: hash},
		},
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	resp, err := uc.Login(context.Background(), "user@example.com", "correct-password")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Token == "" || resp.SessionID != "session-1" || resp.RefreshToken != "refresh-1" {
		t.Fatalf("unexpected login response: %+v", resp)
	}
}

// TestLogin_WrongPasswordAndUnknownEmail_ProduceIdenticalError is the anti-enumeration
// regression test: an existing user with a wrong password and a wholly unknown email must
// resolve to the exact same sentinel error, so a client-facing message can never leak which
// case actually happened.
func TestLogin_WrongPasswordAndUnknownEmail_ProduceIdenticalError(t *testing.T) {
	hash := mustHash(t, "correct-password")
	repo := &fakeSessionRepository{
		usersByEmail: map[string]*domain.User{
			"user@example.com": {ID: "user-1", Email: "user@example.com", PasswordHash: hash},
		},
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	_, wrongPasswordErr := uc.Login(context.Background(), "user@example.com", "wrong-password")
	_, unknownEmailErr := uc.Login(context.Background(), "nobody@example.com", "whatever")

	if !errors.Is(wrongPasswordErr, domain.ErrInvalidCredentials) {
		t.Fatalf("expected wrong password to yield ErrInvalidCredentials, got %v", wrongPasswordErr)
	}
	if !errors.Is(unknownEmailErr, domain.ErrInvalidCredentials) {
		t.Fatalf("expected unknown email to yield ErrInvalidCredentials, got %v", unknownEmailErr)
	}
	if wrongPasswordErr != unknownEmailErr {
		t.Fatalf("expected both failure modes to return the identical sentinel value: wrongPassword=%v unknownEmail=%v", wrongPasswordErr, unknownEmailErr)
	}
}

// TestRegister_Success proves a new email registers cleanly and returns a populated
// LoginResponse.
func TestRegister_Success(t *testing.T) {
	repo := &fakeSessionRepository{}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	resp, err := uc.Register(context.Background(), "new@example.com", "password123", "newuser")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Token == "" || resp.SessionID != "session-1" {
		t.Fatalf("unexpected register response: %+v", resp)
	}
	if repo.addedUser == nil || repo.addedUser.Email != "new@example.com" {
		t.Fatalf("expected AddUser to be called with the new email, got %+v", repo.addedUser)
	}
	if repo.addedUser.PasswordHash == "password123" {
		t.Fatalf("expected password to be hashed before storage, got plaintext")
	}
}

// TestRegister_DuplicateEmail proves an already-registered email is rejected with
// ErrEmailAlreadyRegistered without attempting to create a duplicate account.
func TestRegister_DuplicateEmail(t *testing.T) {
	repo := &fakeSessionRepository{
		usersByEmail: map[string]*domain.User{
			"existing@example.com": {ID: "user-1", Email: "existing@example.com"},
		},
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	_, err := uc.Register(context.Background(), "existing@example.com", "password123", "someone")
	if !errors.Is(err, domain.ErrEmailAlreadyRegistered) {
		t.Fatalf("expected ErrEmailAlreadyRegistered, got %v", err)
	}
	if repo.addedUser != nil {
		t.Fatalf("expected AddUser to never be called for a duplicate email")
	}
}

// TestRefresh_Success proves a valid sessionID+refreshToken pair returns a new token pair.
func TestRefresh_Success(t *testing.T) {
	refreshToken := "correct-refresh-token"
	hash := mustHash(t, refreshToken)
	repo := &fakeSessionRepository{
		sessionsByID: map[string]*domain.AuthSession{
			"session-1": {ID: "session-1", UserID: "user-1", RefreshToken: hash},
		},
		usersByID: map[string]*domain.User{
			"user-1": {ID: "user-1", Email: "user@example.com"},
		},
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	resp, err := uc.Refresh(context.Background(), refreshToken, "session-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Token == "" || resp.RefreshToken != "new-refresh-token" {
		t.Fatalf("unexpected refresh response: %+v", resp)
	}
	if repo.refreshCalls != 1 {
		t.Fatalf("expected RefreshToken to be called exactly once, got %d", repo.refreshCalls)
	}
}

// TestRefresh_UnknownSession proves a sessionID that resolves to no session at all yields
// ErrInvalidSession.
func TestRefresh_UnknownSession(t *testing.T) {
	repo := &fakeSessionRepository{}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	_, err := uc.Refresh(context.Background(), "any-token", "missing-session")
	if !errors.Is(err, domain.ErrInvalidSession) {
		t.Fatalf("expected ErrInvalidSession, got %v", err)
	}
}

// TestRefresh_WrongRefreshToken proves a session that exists but whose stored refresh token
// hash does not match the supplied token yields the distinct ErrInvalidToken (not
// ErrInvalidSession) - a session ID is not itself a guessable account identifier, so this
// case is allowed to be distinguishable from "unknown session", unlike Login's anti-enumeration
// requirement.
func TestRefresh_WrongRefreshToken(t *testing.T) {
	hash := mustHash(t, "correct-refresh-token")
	repo := &fakeSessionRepository{
		sessionsByID: map[string]*domain.AuthSession{
			"session-1": {ID: "session-1", UserID: "user-1", RefreshToken: hash},
		},
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	_, err := uc.Refresh(context.Background(), "wrong-refresh-token", "session-1")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

// TestRefresh_UserMissing proves that when the session's owning user can no longer be found
// (e.g. deleted), Refresh reports ErrInvalidSession rather than leaking an internal error.
func TestRefresh_UserMissing(t *testing.T) {
	refreshToken := "correct-refresh-token"
	hash := mustHash(t, refreshToken)
	repo := &fakeSessionRepository{
		sessionsByID: map[string]*domain.AuthSession{
			"session-1": {ID: "session-1", UserID: "user-1", RefreshToken: hash},
		},
		// usersByID intentionally left nil/empty: GetUserByID resolves to (nil, nil).
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	_, err := uc.Refresh(context.Background(), refreshToken, "session-1")
	if !errors.Is(err, domain.ErrInvalidSession) {
		t.Fatalf("expected ErrInvalidSession when the session's user no longer exists, got %v", err)
	}
}

// TestAccessToken_CarriesRoleFromDatabase proves login and refresh issue the account's role in
// the `role` claim, read from the database each time (a promotion applies at the next refresh),
// and that an empty or unknown stored role is issued as "user", never as a privilege.
func TestAccessToken_CarriesRoleFromDatabase(t *testing.T) {
	verifier, err := authjwt.NewVerifier(testJWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	roleOf := func(token string) string {
		t.Helper()
		claims, err := verifier.VerifyClaims(token)
		if err != nil {
			t.Fatalf("issued token does not verify: %v", err)
		}
		return claims.Role
	}

	hash := mustHash(t, "correct-password")
	for stored, want := range map[domain.UserRole]string{
		domain.RoleAdmin: authjwt.RoleAdmin,
		domain.RoleUser:  authjwt.RoleUser,
		"":               authjwt.RoleUser,
		"superuser":      authjwt.RoleUser,
	} {
		repo := &fakeSessionRepository{usersByEmail: map[string]*domain.User{
			"user@example.com": {ID: "user-1", Email: "user@example.com", PasswordHash: hash, Role: stored},
		}}
		resp, err := NewAuthQueryUseCase(repo, testJWTSecret).Login(context.Background(), "user@example.com", "correct-password")
		if err != nil {
			t.Fatalf("login (role %q): %v", stored, err)
		}
		if got := roleOf(resp.Token); got != want {
			t.Fatalf("login with stored role %q: claim = %q, want %q", stored, got, want)
		}
	}

	refreshToken := "correct-refresh-token"
	repo := &fakeSessionRepository{
		sessionsByID: map[string]*domain.AuthSession{
			"session-1": {ID: "session-1", UserID: "user-1", RefreshToken: mustHash(t, refreshToken)},
		},
		usersByID: map[string]*domain.User{
			"user-1": {ID: "user-1", Email: "user@example.com", Role: domain.RoleAdmin},
		},
	}
	resp, err := NewAuthQueryUseCase(repo, testJWTSecret).Refresh(context.Background(), refreshToken, "session-1")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := roleOf(resp.Token); got != authjwt.RoleAdmin {
		t.Fatalf("refresh after promotion: claim = %q, want admin", got)
	}
}

// TestJWTRoundTrip_SameSecret proves a token generated by generateJWT can be verified by
// VerifyToken against the same secret, and returns the exact user ID that was signed in.
func TestJWTRoundTrip_SameSecret(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, testJWTSecret)

	token, err := uc.generateJWT(&domain.User{ID: "user-42", Email: "user42@example.com"})
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	userID, err := uc.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if userID != "user-42" {
		t.Fatalf("expected userID 'user-42', got %q", userID)
	}
}

// TestJWTRoundTrip_DifferentSecret is the direct regression test for the removed hardcoded
// "dev-secret" fallback: a token signed with one instance's secret must be rejected by an
// instance configured with a different secret. Without a per-environment required
// AUTH_JWT_SECRET, two differently-configured instances could otherwise forge each other's
// tokens.
func TestJWTRoundTrip_DifferentSecret(t *testing.T) {
	issuer := NewAuthQueryUseCase(&fakeSessionRepository{}, "issuer-secret-long-enough-for-hs256-keys")
	verifier := NewAuthQueryUseCase(&fakeSessionRepository{}, "verifier-secret-long-enough-for-hs256-key")

	token, err := issuer.generateJWT(&domain.User{ID: "user-1", Email: "user@example.com"})
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	if _, err := verifier.VerifyToken(token); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("expected a token signed with a different secret to be rejected with ErrInvalidToken, got %v", err)
	}
}

// TestVerifyToken_MalformedInputsRejectedWithoutPanic proves VerifyToken handles garbage
// input safely - an empty string, a non-JWT string, and a token signed with an unexpected
// algorithm (alg confusion) must all be rejected, not panic.
func TestVerifyToken_MalformedInputsRejectedWithoutPanic(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, testJWTSecret)

	cases := map[string]string{
		"empty string":   "",
		"garbage string": "not-a-jwt-at-all",
		"two-part token": "onlyonedot.here",
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("VerifyToken panicked on input %q: %v", token, r)
				}
			}()
			if _, err := uc.VerifyToken(token); err == nil {
				t.Fatalf("expected an error for malformed token %q, got nil", token)
			}
		})
	}
}

// TestVerifyToken_RejectsUnexpectedSigningAlgorithm proves a token signed with a different
// algorithm than the one VerifyToken restricts to (HS256) is rejected, guarding against
// alg-confusion style attacks.
func TestVerifyToken_RejectsUnexpectedSigningAlgorithm(t *testing.T) {
	uc := NewAuthQueryUseCase(&fakeSessionRepository{}, testJWTSecret)

	claims := jwt.MapClaims{"sub": "user-1"}
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign token with HS384: %v", err)
	}

	if _, err := uc.VerifyToken(signed); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for an unexpected signing algorithm, got %v", err)
	}
}

// TestHashPassword_RoundTrip proves hashPassword produces a hash distinct from the input
// plaintext, and that bcrypt.CompareHashAndPassword against the original password succeeds.
func TestHashPassword_RoundTrip(t *testing.T) {
	hash, err := hashPassword("my-secret-password")
	if err != nil {
		t.Fatalf("hashPassword failed: %v", err)
	}
	if hash == "my-secret-password" {
		t.Fatalf("expected hash to differ from plaintext input")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("my-secret-password")); err != nil {
		t.Fatalf("expected the generated hash to validate against the original password: %v", err)
	}
}

// TestHashPassword_TooLongPasswordErrors proves hashPassword surfaces bcrypt's real error for
// a password exceeding its 72-byte limit, rather than silently truncating or panicking.
func TestHashPassword_TooLongPasswordErrors(t *testing.T) {
	tooLong := strings.Repeat("a", 100)
	if _, err := hashPassword(tooLong); err == nil {
		t.Fatalf("expected an error for a password exceeding bcrypt's length limit")
	}
}

// TestLogin_AddUserSessionFailure proves a repository failure while creating the session (after
// credentials already validated) is propagated to the caller.
func TestLogin_AddUserSessionFailure(t *testing.T) {
	hash := mustHash(t, "correct-password")
	repo := &fakeSessionRepository{
		usersByEmail: map[string]*domain.User{
			"user@example.com": {ID: "user-1", Email: "user@example.com", PasswordHash: hash},
		},
		addUserSessionErr: errors.New("db unavailable"),
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	if _, err := uc.Login(context.Background(), "user@example.com", "correct-password"); err == nil {
		t.Fatalf("expected the repository error to propagate")
	}
}

// TestRegister_GetUserByEmailFailure proves a repository failure while checking for an
// existing email is propagated rather than treated as "email available".
func TestRegister_GetUserByEmailFailure(t *testing.T) {
	repo := &fakeSessionRepository{getUserByEmailErr: errors.New("db unavailable")}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	if _, err := uc.Register(context.Background(), "new@example.com", "pw", "name"); err == nil {
		t.Fatalf("expected the repository error to propagate")
	}
}

// TestRegister_AddUserFailure proves a repository failure while creating the new user is
// propagated to the caller.
func TestRegister_AddUserFailure(t *testing.T) {
	repo := &fakeSessionRepository{addUserErr: errors.New("unique constraint violated")}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	if _, err := uc.Register(context.Background(), "new@example.com", "pw", "name"); err == nil {
		t.Fatalf("expected the repository error to propagate")
	}
}

// TestRefresh_RefreshTokenFailure proves a repository failure while rotating the refresh
// token is propagated to the caller.
func TestRefresh_RefreshTokenFailure(t *testing.T) {
	refreshToken := "correct-refresh-token"
	hash := mustHash(t, refreshToken)
	repo := &fakeSessionRepository{
		sessionsByID: map[string]*domain.AuthSession{
			"session-1": {ID: "session-1", UserID: "user-1", RefreshToken: hash},
		},
		refreshTokenErr: errors.New("db unavailable"),
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	if _, err := uc.Refresh(context.Background(), refreshToken, "session-1"); err == nil {
		t.Fatalf("expected the repository error to propagate")
	}
}

// TestRefresh_GetUserByIDFailure proves a repository failure while re-fetching the session's
// owning user is propagated to the caller.
func TestRefresh_GetUserByIDFailure(t *testing.T) {
	refreshToken := "correct-refresh-token"
	hash := mustHash(t, refreshToken)
	repo := &fakeSessionRepository{
		sessionsByID: map[string]*domain.AuthSession{
			"session-1": {ID: "session-1", UserID: "user-1", RefreshToken: hash},
		},
		getUserByIDErr: errors.New("db unavailable"),
	}
	uc := NewAuthQueryUseCase(repo, testJWTSecret)

	if _, err := uc.Refresh(context.Background(), refreshToken, "session-1"); err == nil {
		t.Fatalf("expected the repository error to propagate")
	}
}
