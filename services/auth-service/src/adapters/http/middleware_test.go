/**
##
## OverDrive 2026
## All Technical rights reserved
##
## middleware_test.go - Package httpadapter source file for services/auth-service/src/adapters/http.
##
*/

package httpadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// fakeAuthQueryUseCaseForMiddleware implements ports.AuthQueryUseCase. Only VerifyToken needs
// a real implementation for requireOwnUser's tests - the middleware never calls the other 3
// methods, so they panic if invoked, making an accidental call to them fail loudly instead of
// silently.
type fakeAuthQueryUseCaseForMiddleware struct {
	verifyTokenFn func(tokenString string) (string, error)
}

func (f *fakeAuthQueryUseCaseForMiddleware) Login(ctx context.Context, email string, password string) (*domain.LoginResponse, error) {
	panic("not used by requireOwnUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) Register(ctx context.Context, email string, password string, userName string) (*domain.LoginResponse, error) {
	panic("not used by requireOwnUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) Refresh(ctx context.Context, refreshToken string, sessionID string) (*domain.RefreshResponse, error) {
	panic("not used by requireOwnUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) VerifyToken(tokenString string) (string, error) {
	return f.verifyTokenFn(tokenString)
}

// newProtectedTestHandler builds a requireOwnUser-wrapped handler for the given {userID} path
// pattern, plus a pointer that records whether the inner handler was actually reached.
func newProtectedTestHandler(authUseCase *fakeAuthQueryUseCaseForMiddleware) (http.Handler, *bool) {
	reached := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	mux := http.NewServeMux()
	mux.Handle("GET /{userID}/protected", requireOwnUser(authUseCase)(inner))
	return mux, &reached
}

// TestRequireOwnUser_MissingAuthorizationHeader proves a request with no Authorization header
// at all is rejected with 401 and never reaches the protected handler.
func TestRequireOwnUser_MissingAuthorizationHeader(t *testing.T) {
	authUseCase := &fakeAuthQueryUseCaseForMiddleware{
		verifyTokenFn: func(tokenString string) (string, error) {
			panic("VerifyToken must not be called when the header is missing")
		},
	}
	handler, reached := newProtectedTestHandler(authUseCase)

	req := httptest.NewRequest(http.MethodGet, "/user-1/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if *reached {
		t.Fatalf("expected the protected handler to never be reached")
	}
}

// TestRequireOwnUser_InvalidToken proves a request whose bearer token fails VerifyToken is
// rejected with 401 and never reaches the protected handler.
func TestRequireOwnUser_InvalidToken(t *testing.T) {
	authUseCase := &fakeAuthQueryUseCaseForMiddleware{
		verifyTokenFn: func(tokenString string) (string, error) {
			return "", domain.ErrInvalidToken
		},
	}
	handler, reached := newProtectedTestHandler(authUseCase)

	req := httptest.NewRequest(http.MethodGet, "/user-1/protected", nil)
	req.Header.Set("Authorization", "Bearer some-invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if *reached {
		t.Fatalf("expected the protected handler to never be reached")
	}
}

// TestRequireOwnUser_TokenAuthenticatesDifferentUser is the direct regression test for the
// IDOR that was fixed in requireOwnUser: a valid token that authenticates as a DIFFERENT user
// than the {userID} path segment being accessed must be rejected with 403, not allowed
// through. Before this fix, any caller holding a valid JWT for ANY account could read/add/
// remove session records for every OTHER user.
func TestRequireOwnUser_TokenAuthenticatesDifferentUser(t *testing.T) {
	authUseCase := &fakeAuthQueryUseCaseForMiddleware{
		verifyTokenFn: func(tokenString string) (string, error) {
			return "user-attacker", nil
		},
	}
	handler, reached := newProtectedTestHandler(authUseCase)

	req := httptest.NewRequest(http.MethodGet, "/user-victim/protected", nil)
	req.Header.Set("Authorization", "Bearer attacker-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when the token authenticates a different user than the path's {userID}, got %d", rec.Code)
	}
	if *reached {
		t.Fatalf("expected the protected handler to never be reached for a cross-user request - this is the IDOR requireOwnUser exists to close")
	}
}

// TestRequireOwnUser_TokenAuthenticatesSameUser proves a valid token whose authenticated user
// matches the {userID} path segment is allowed through to the protected handler.
func TestRequireOwnUser_TokenAuthenticatesSameUser(t *testing.T) {
	authUseCase := &fakeAuthQueryUseCaseForMiddleware{
		verifyTokenFn: func(tokenString string) (string, error) {
			return "user-1", nil
		},
	}
	handler, reached := newProtectedTestHandler(authUseCase)

	req := httptest.NewRequest(http.MethodGet, "/user-1/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !*reached {
		t.Fatalf("expected the protected handler to be reached for a matching user")
	}
}

// TestWithSecurityHeaders proves the baseline hardening headers are applied to every response.
func TestWithSecurityHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := withSecurityHeaders(inner)

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	checks := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
	}
	for header, want := range checks {
		if got := rec.Header().Get(header); got != want {
			t.Fatalf("expected header %s=%q, got %q", header, want, got)
		}
	}
}
