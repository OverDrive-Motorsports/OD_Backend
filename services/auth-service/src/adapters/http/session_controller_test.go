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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"overdrive/services/auth-service/src/core/domain"

	"github.com/steebchen/prisma-client-go/runtime/types"
)

// fakeSessionQueryUseCase implements ports.SessionQueryUseCase for SessionController tests and
// records the IDs each call received.
type fakeSessionQueryUseCase struct {
	sessions  []domain.AuthSession
	session   *domain.AuthSession
	err       error
	removeErr error

	gotUserID    string
	gotSessionID string
}

func (f *fakeSessionQueryUseCase) GetUserSessions(ctx context.Context, userID string) ([]domain.AuthSession, error) {
	f.gotUserID = userID
	return f.sessions, f.err
}

func (f *fakeSessionQueryUseCase) GetUserSession(ctx context.Context, userID string, sessionID string) (*domain.AuthSession, error) {
	f.gotUserID, f.gotSessionID = userID, sessionID
	return f.session, f.err
}

func (f *fakeSessionQueryUseCase) GetSessionByID(ctx context.Context, sessionID string) (*domain.AuthSession, error) {
	return f.session, f.err
}

func (f *fakeSessionQueryUseCase) RemoveUserSession(ctx context.Context, userID string, sessionID string) error {
	f.gotUserID, f.gotSessionID = userID, sessionID
	return f.removeErr
}

// serveSessions sends one request through the real router, authenticated as user-1 when
// authorization is "Bearer token-user-1" (any other token is rejected).
func serveSessions(t *testing.T, usecase *fakeSessionQueryUseCase, method, path, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	authUseCase := &fakeAuthQueryUseCaseForMiddleware{verifyTokenFn: func(token string) (string, error) {
		if token == "token-user-1" {
			return "user-1", nil
		}
		return "", domain.ErrInvalidToken
	}}
	router := NewRouter(NewHealthController(nil), NewSessionController(usecase), NewAuthController(nil), authUseCase)
	req := httptest.NewRequest(method, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func storedSession(id string) domain.AuthSession {
	at := types.DateTime(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
	return domain.AuthSession{ID: id, UserID: "user-1", RefreshToken: "$2a$10$stored-hash", CreatedAt: at, ExpiresAt: at}
}

// TestSessionRoutes_ListReturnsCallerSessionsAsBareArray proves GET /sessions acts on the token's
// user, answers a bare array of {id, createdAt, expiresAt}, and never exposes the refresh token
// or its hash.
func TestSessionRoutes_ListReturnsCallerSessionsAsBareArray(t *testing.T) {
	usecase := &fakeSessionQueryUseCase{sessions: []domain.AuthSession{storedSession("s1"), storedSession("s2")}}
	rec := serveSessions(t, usecase, http.MethodGet, "/sessions", "Bearer token-user-1")

	if rec.Code != http.StatusOK || usecase.gotUserID != "user-1" {
		t.Fatalf("status = %d, usecase user = %q", rec.Code, usecase.gotUserID)
	}
	var body []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body) != 2 {
		t.Fatalf("expected a bare array of 2 sessions, got %s (%v)", rec.Body.String(), err)
	}
	for _, field := range []string{"id", "createdAt", "expiresAt"} {
		if _, ok := body[0][field]; !ok {
			t.Fatalf("missing %s in %v", field, body[0])
		}
	}
	if len(body[0]) != 3 || strings.Contains(rec.Body.String(), "stored-hash") {
		t.Fatalf("session view must only carry id, createdAt, expiresAt: %s", rec.Body.String())
	}
}

// TestSessionRoutes_ListEmptyIsArray proves a user without sessions gets [] and not null.
func TestSessionRoutes_ListEmptyIsArray(t *testing.T) {
	rec := serveSessions(t, &fakeSessionQueryUseCase{}, http.MethodGet, "/sessions", "Bearer token-user-1")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("status = %d body = %s, want 200 []", rec.Code, rec.Body.String())
	}
}

// TestSessionRoutes_GetOne proves GET /sessions/{sessionId} passes the token's user and the path
// session to the usecase and answers the session view without the refresh token hash.
func TestSessionRoutes_GetOne(t *testing.T) {
	session := storedSession("s1")
	usecase := &fakeSessionQueryUseCase{session: &session}
	rec := serveSessions(t, usecase, http.MethodGet, "/sessions/s1", "Bearer token-user-1")

	if rec.Code != http.StatusOK || usecase.gotUserID != "user-1" || usecase.gotSessionID != "s1" {
		t.Fatalf("status = %d, usecase got user %q session %q", rec.Code, usecase.gotUserID, usecase.gotSessionID)
	}
	if !strings.Contains(rec.Body.String(), `"id":"s1"`) || strings.Contains(rec.Body.String(), "stored-hash") {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

// TestSessionRoutes_GetOneNotFound proves an unknown session, or another user's (the repository
// filters on the owner and finds nothing), answers 404 SESSION_NOT_FOUND.
func TestSessionRoutes_GetOneNotFound(t *testing.T) {
	rec := serveSessions(t, &fakeSessionQueryUseCase{}, http.MethodGet, "/sessions/other-users", "Bearer token-user-1")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"SESSION_NOT_FOUND"`) {
		t.Fatalf("status = %d body = %s, want 404 SESSION_NOT_FOUND", rec.Code, rec.Body.String())
	}
}

// TestSessionRoutes_Delete proves DELETE /sessions/{sessionId} revokes the caller's session with
// 204 and an empty body, and maps a session the caller does not own to 404.
func TestSessionRoutes_Delete(t *testing.T) {
	usecase := &fakeSessionQueryUseCase{}
	rec := serveSessions(t, usecase, http.MethodDelete, "/sessions/s1", "Bearer token-user-1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || usecase.gotUserID != "user-1" || usecase.gotSessionID != "s1" {
		t.Fatalf("status = %d body = %q, usecase got user %q session %q", rec.Code, rec.Body.String(), usecase.gotUserID, usecase.gotSessionID)
	}

	rec = serveSessions(t, &fakeSessionQueryUseCase{removeErr: domain.ErrSessionNotFound}, http.MethodDelete, "/sessions/s9", "Bearer token-user-1")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"SESSION_NOT_FOUND"`) {
		t.Fatalf("not owned: status = %d body = %s, want 404 SESSION_NOT_FOUND", rec.Code, rec.Body.String())
	}
}

// TestSessionRoutes_InternalErrorsDoNotLeak proves a usecase failure on each route answers 500
// with the generic message, not the technical cause.
func TestSessionRoutes_InternalErrorsDoNotLeak(t *testing.T) {
	cause := errors.New("pq: connection refused")
	for name, tc := range map[string]struct {
		method, path string
		usecase      *fakeSessionQueryUseCase
	}{
		"list":   {http.MethodGet, "/sessions", &fakeSessionQueryUseCase{err: cause}},
		"get":    {http.MethodGet, "/sessions/s1", &fakeSessionQueryUseCase{err: cause}},
		"delete": {http.MethodDelete, "/sessions/s1", &fakeSessionQueryUseCase{removeErr: cause}},
	} {
		rec := serveSessions(t, tc.usecase, tc.method, tc.path, "Bearer token-user-1")
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "connection refused") {
			t.Fatalf("%s: status = %d body = %s", name, rec.Code, rec.Body.String())
		}
	}
}

// TestSessionRoutes_RequireToken proves every session route refuses a request without a valid
// token before reaching the usecase, and that the old /{userID}/session(s) routes and the
// session creation route no longer exist.
func TestSessionRoutes_RequireToken(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/sessions"},
		{http.MethodGet, "/sessions/s1"},
		{http.MethodDelete, "/sessions/s1"},
	} {
		usecase := &fakeSessionQueryUseCase{}
		for _, authorization := range []string{"", "Bearer forged"} {
			rec := serveSessions(t, usecase, tc.method, tc.path, authorization)
			if rec.Code != http.StatusUnauthorized || usecase.gotUserID != "" {
				t.Fatalf("%s %s with %q: status = %d, usecase reached as %q", tc.method, tc.path, authorization, rec.Code, usecase.gotUserID)
			}
		}
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/user-1/sessions"},
		{http.MethodGet, "/user-1/session/s1"},
		{http.MethodPost, "/user-1/session"},
		{http.MethodDelete, "/user-1/session/s1"},
	} {
		rec := serveSessions(t, &fakeSessionQueryUseCase{}, tc.method, tc.path, "Bearer token-user-1")
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s must no longer exist, got %d", tc.method, tc.path, rec.Code)
		}
	}
	if rec := serveSessions(t, &fakeSessionQueryUseCase{}, http.MethodPost, "/sessions", "Bearer token-user-1"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /sessions must not create a session, got %d", rec.Code)
	}
}
