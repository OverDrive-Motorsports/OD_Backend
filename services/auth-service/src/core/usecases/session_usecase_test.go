/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## session_usecase_test.go - Package usecases source file for services/auth-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"
	"errors"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// recordingSessionRepository is a ports.SessionRepository fake that records the arguments it
// was called with, and returns whatever canned values/errors are configured, so
// SessionQueryUseCase's pass-through behavior can be verified precisely.
type recordingSessionRepository struct {
	gotUserID    string
	gotSessionID string

	sessions    []domain.AuthSession
	session     *domain.AuthSession
	err         error
	removeErr   error
	removeCalls int
}

func (f *recordingSessionRepository) GetUserSessions(ctx context.Context, userID string) ([]domain.AuthSession, error) {
	f.gotUserID = userID
	return f.sessions, f.err
}

func (f *recordingSessionRepository) GetUserSession(ctx context.Context, userID string, sessionID string) (*domain.AuthSession, error) {
	f.gotUserID = userID
	f.gotSessionID = sessionID
	return f.session, f.err
}

func (f *recordingSessionRepository) GetSessionByID(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	f.gotSessionID = sessionID
	return f.session, f.err
}

func (f *recordingSessionRepository) AddUserSession(ctx context.Context, userID string) (*domain.AuthSession, error) {
	f.gotUserID = userID
	return f.session, f.err
}

func (f *recordingSessionRepository) RemoveUserSession(ctx context.Context, userID string, sessionID string) error {
	f.removeCalls++
	f.gotUserID = userID
	f.gotSessionID = sessionID
	return f.removeErr
}

func (f *recordingSessionRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	panic("not used by SessionQueryUseCase")
}

func (f *recordingSessionRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	panic("not used by SessionQueryUseCase")
}

func (f *recordingSessionRepository) AddUser(ctx context.Context, email string, passwordHash string, username string) (*domain.User, error) {
	panic("not used by SessionQueryUseCase")
}

func (f *recordingSessionRepository) RefreshToken(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	panic("not used by SessionQueryUseCase")
}

// TestSessionQueryUseCase_GetUserSessions proves the userID argument and the repository's
// result/error are forwarded unchanged.
func TestSessionQueryUseCase_GetUserSessions(t *testing.T) {
	want := []domain.AuthSession{{ID: "s1"}, {ID: "s2"}}
	repo := &recordingSessionRepository{sessions: want}
	uc := NewSessionQueryUseCase(repo)

	got, err := uc.GetUserSessions(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.gotUserID != "user-1" {
		t.Fatalf("expected userID to be forwarded, got %q", repo.gotUserID)
	}
	if len(got) != 2 {
		t.Fatalf("expected the repository's sessions to be returned unchanged, got %+v", got)
	}
}

// TestSessionQueryUseCase_GetUserSessions_PropagatesError proves a repository failure is
// surfaced to the caller rather than swallowed.
func TestSessionQueryUseCase_GetUserSessions_PropagatesError(t *testing.T) {
	repo := &recordingSessionRepository{err: errors.New("db unavailable")}
	uc := NewSessionQueryUseCase(repo)

	if _, err := uc.GetUserSessions(context.Background(), "user-1"); err == nil {
		t.Fatalf("expected the repository error to propagate")
	}
}

// TestSessionQueryUseCase_GetUserSession proves both userID and sessionID are forwarded.
func TestSessionQueryUseCase_GetUserSession(t *testing.T) {
	want := &domain.AuthSession{ID: "s1"}
	repo := &recordingSessionRepository{session: want}
	uc := NewSessionQueryUseCase(repo)

	got, err := uc.GetUserSession(context.Background(), "user-1", "s1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.gotUserID != "user-1" || repo.gotSessionID != "s1" {
		t.Fatalf("expected both IDs to be forwarded, got userID=%q sessionID=%q", repo.gotUserID, repo.gotSessionID)
	}
	if got != want {
		t.Fatalf("expected the repository's result to be returned unchanged")
	}
}

// TestSessionQueryUseCase_GetSessionByID proves the sessionID is forwarded and the result
// passed through.
func TestSessionQueryUseCase_GetSessionByID(t *testing.T) {
	want := &domain.AuthSession{ID: "s7"}
	repo := &recordingSessionRepository{session: want}
	uc := NewSessionQueryUseCase(repo)

	got, err := uc.GetSessionByID(context.Background(), "s7")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.gotSessionID != "s7" {
		t.Fatalf("expected sessionID to be forwarded, got %q", repo.gotSessionID)
	}
	if got != want {
		t.Fatalf("expected the repository's result to be returned unchanged")
	}
}

// TestSessionQueryUseCase_AddUserSession proves the userID is forwarded and the created
// session passed through.
func TestSessionQueryUseCase_AddUserSession(t *testing.T) {
	want := &domain.AuthSession{ID: "new-session"}
	repo := &recordingSessionRepository{session: want}
	uc := NewSessionQueryUseCase(repo)

	got, err := uc.AddUserSession(context.Background(), "user-9")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.gotUserID != "user-9" {
		t.Fatalf("expected userID to be forwarded, got %q", repo.gotUserID)
	}
	if got != want {
		t.Fatalf("expected the repository's result to be returned unchanged")
	}
}

// TestSessionQueryUseCase_RemoveUserSession proves both IDs are forwarded and the
// repository's error (or lack thereof) is passed through.
func TestSessionQueryUseCase_RemoveUserSession(t *testing.T) {
	repo := &recordingSessionRepository{}
	uc := NewSessionQueryUseCase(repo)

	if err := uc.RemoveUserSession(context.Background(), "user-1", "s1"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.removeCalls != 1 || repo.gotUserID != "user-1" || repo.gotSessionID != "s1" {
		t.Fatalf("expected exactly one forwarded call with both IDs, got calls=%d userID=%q sessionID=%q", repo.removeCalls, repo.gotUserID, repo.gotSessionID)
	}
}

// TestSessionQueryUseCase_RemoveUserSession_PropagatesError proves a repository failure is
// surfaced rather than swallowed.
func TestSessionQueryUseCase_RemoveUserSession_PropagatesError(t *testing.T) {
	repo := &recordingSessionRepository{removeErr: errors.New("delete failed")}
	uc := NewSessionQueryUseCase(repo)

	if err := uc.RemoveUserSession(context.Background(), "user-1", "s1"); err == nil {
		t.Fatalf("expected the repository error to propagate")
	}
}
