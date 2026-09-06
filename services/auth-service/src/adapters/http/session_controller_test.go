/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## session_controller_test.go - Package httpadapter source file for services/auth-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// fakeSessionQueryUseCase implements ports.SessionQueryUseCase for SessionController tests.
type fakeSessionQueryUseCase struct {
	sessions  []domain.AuthSession
	session   *domain.AuthSession
	err       error
	removeErr error
}

func (f *fakeSessionQueryUseCase) GetUserSessions(ctx context.Context, userID string) ([]domain.AuthSession, error) {
	return f.sessions, f.err
}

func (f *fakeSessionQueryUseCase) GetUserSession(ctx context.Context, userID string, sessionID string) (*domain.AuthSession, error) {
	return f.session, f.err
}

func (f *fakeSessionQueryUseCase) GetSessionByID(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	return f.session, f.err
}

func (f *fakeSessionQueryUseCase) AddUserSession(ctx context.Context, userID string) (*domain.AuthSession, error) {
	return f.session, f.err
}

func (f *fakeSessionQueryUseCase) RemoveUserSession(ctx context.Context, userID string, sessionID string) error {
	return f.removeErr
}

func newRequestWithPathValues(method, path string, values map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range values {
		req.SetPathValue(k, v)
	}
	return req
}

// TestSessionController_GetUserSessions_Success proves the normal case returns 200 with the
// usecase's sessions.
func TestSessionController_GetUserSessions_Success(t *testing.T) {
	usecase := &fakeSessionQueryUseCase{sessions: []domain.AuthSession{{ID: "s1"}}}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodGet, "/user-1/sessions", map[string]string{"userID": "user-1"})
	rec := httptest.NewRecorder()
	controller.GetUserSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// TestSessionController_GetUserSessions_GenericFailure proves a repository failure is a 500
// without leaking its technical detail.
func TestSessionController_GetUserSessions_GenericFailure(t *testing.T) {
	technicalDetail := "pq: too many connections"
	usecase := &fakeSessionQueryUseCase{err: errors.New(technicalDetail)}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodGet, "/user-1/sessions", map[string]string{"userID": "user-1"})
	rec := httptest.NewRecorder()
	controller.GetUserSessions(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), technicalDetail) {
		t.Fatalf("expected the technical error detail to never leak into the response body, got %s", rec.Body.String())
	}
}

// TestSessionController_GetUserSession_Success proves the normal case returns 200 with the
// found session.
func TestSessionController_GetUserSession_Success(t *testing.T) {
	usecase := &fakeSessionQueryUseCase{session: &domain.AuthSession{ID: "s1"}}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodGet, "/user-1/session/s1", map[string]string{"userID": "user-1", "sessionID": "s1"})
	rec := httptest.NewRecorder()
	controller.GetUserSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// TestSessionController_GetUserSession_NotFound proves a nil (not-found) result from the
// usecase maps to a 404, not a 200 with an empty body or a 500.
func TestSessionController_GetUserSession_NotFound(t *testing.T) {
	usecase := &fakeSessionQueryUseCase{session: nil}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodGet, "/user-1/session/missing", map[string]string{"userID": "user-1", "sessionID": "missing"})
	rec := httptest.NewRecorder()
	controller.GetUserSession(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestSessionController_GetUserSession_GenericFailure proves a repository failure is a 500
// without leaking its technical detail.
func TestSessionController_GetUserSession_GenericFailure(t *testing.T) {
	technicalDetail := "prisma: unexpected EOF from connection"
	usecase := &fakeSessionQueryUseCase{err: errors.New(technicalDetail)}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodGet, "/user-1/session/s1", map[string]string{"userID": "user-1", "sessionID": "s1"})
	rec := httptest.NewRecorder()
	controller.GetUserSession(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), technicalDetail) {
		t.Fatalf("expected the technical error detail to never leak into the response body, got %s", rec.Body.String())
	}
}

// TestSessionController_AddUserSession_Success proves the normal case returns 200 with the
// newly created session.
func TestSessionController_AddUserSession_Success(t *testing.T) {
	usecase := &fakeSessionQueryUseCase{session: &domain.AuthSession{ID: "new-session"}}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodPost, "/user-1/session", map[string]string{"userID": "user-1"})
	rec := httptest.NewRecorder()
	controller.AddUserSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// TestSessionController_AddUserSession_GenericFailure proves a repository failure is a 500
// without leaking its technical detail.
func TestSessionController_AddUserSession_GenericFailure(t *testing.T) {
	technicalDetail := "prisma: foreign key violation on userID"
	usecase := &fakeSessionQueryUseCase{err: errors.New(technicalDetail)}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodPost, "/user-1/session", map[string]string{"userID": "user-1"})
	rec := httptest.NewRecorder()
	controller.AddUserSession(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), technicalDetail) {
		t.Fatalf("expected the technical error detail to never leak into the response body, got %s", rec.Body.String())
	}
}

// TestSessionController_RemoveUserSession_Success proves the normal case returns 200.
func TestSessionController_RemoveUserSession_Success(t *testing.T) {
	usecase := &fakeSessionQueryUseCase{}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodDelete, "/user-1/session/s1", map[string]string{"userID": "user-1", "sessionID": "s1"})
	rec := httptest.NewRecorder()
	controller.RemoveUserSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// TestSessionController_RemoveUserSession_GenericFailure proves a repository failure is a 500
// without leaking its technical detail.
func TestSessionController_RemoveUserSession_GenericFailure(t *testing.T) {
	technicalDetail := "prisma: connection reset by peer"
	usecase := &fakeSessionQueryUseCase{removeErr: errors.New(technicalDetail)}
	controller := NewSessionController(usecase)

	req := newRequestWithPathValues(http.MethodDelete, "/user-1/session/s1", map[string]string{"userID": "user-1", "sessionID": "s1"})
	rec := httptest.NewRecorder()
	controller.RemoveUserSession(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), technicalDetail) {
		t.Fatalf("expected the technical error detail to never leak into the response body, got %s", rec.Body.String())
	}
}
