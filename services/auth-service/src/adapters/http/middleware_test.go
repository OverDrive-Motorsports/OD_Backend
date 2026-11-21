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
	"strings"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// fakeAuthQueryUseCaseForMiddleware implements ports.AuthQueryUseCase. Only VerifyToken needs
// a real implementation for requireUser's tests - the middleware never calls the other 3
// methods, so they panic if invoked, making an accidental call to them fail loudly instead of
// silently.
type fakeAuthQueryUseCaseForMiddleware struct {
	verifyTokenFn func(tokenString string) (string, error)
}

func (f *fakeAuthQueryUseCaseForMiddleware) Login(ctx context.Context, email string, password string) (*domain.LoginResponse, error) {
	panic("not used by requireUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) Register(ctx context.Context, email string, password string, userName string) (*domain.LoginResponse, error) {
	panic("not used by requireUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) Refresh(ctx context.Context, refreshToken string, sessionID string) (*domain.RefreshResponse, error) {
	panic("not used by requireUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) ListOAuthProviders() []domain.OAuthProviderInfo {
	panic("not used by requireUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) OAuthAuthorize(providerID string) (*domain.OAuthAuthorization, error) {
	panic("not used by requireUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) OAuthLogin(ctx context.Context, providerID string, callback domain.OAuthCallback) (*domain.LoginResponse, error) {
	panic("not used by requireUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) OAuthRegister(ctx context.Context, providerID string, callback domain.OAuthCallback, username string) (*domain.LoginResponse, error) {
	panic("not used by requireUser")
}

func (f *fakeAuthQueryUseCaseForMiddleware) VerifyToken(tokenString string) (string, error) {
	return f.verifyTokenFn(tokenString)
}

// newProtectedTestHandler builds a requireUser-wrapped handler, plus a pointer that records the
// user ID the inner handler read from the context (empty when it was never reached).
func newProtectedTestHandler(authUseCase *fakeAuthQueryUseCaseForMiddleware) (http.Handler, *string) {
	reachedAs := ""
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedAs = userIDFrom(r)
		w.WriteHeader(http.StatusOK)
	})

	mux := http.NewServeMux()
	mux.Handle("GET /protected", requireUser(authUseCase)(inner))
	return mux, &reachedAs
}

// TestRequireUser proves the middleware answers 401 UNAUTHORIZED without a bearer token and
// 401 INVALID_TOKEN for a token VerifyToken rejects, without reaching the handler, and that an
// accepted token reaches it with the token's user ID in the context (the bearer scheme is
// matched case-insensitively).
func TestRequireUser(t *testing.T) {
	authUseCase := &fakeAuthQueryUseCaseForMiddleware{verifyTokenFn: func(token string) (string, error) {
		if token == "good-token" {
			return "user-1", nil
		}
		return "", domain.ErrInvalidToken
	}}

	for name, tc := range map[string]struct {
		authorization, wantCode, wantUser string
		wantStatus                        int
	}{
		"no header":     {"", `"UNAUTHORIZED"`, "", http.StatusUnauthorized},
		"other scheme":  {"Basic good-token", `"UNAUTHORIZED"`, "", http.StatusUnauthorized},
		"empty token":   {"Bearer ", `"UNAUTHORIZED"`, "", http.StatusUnauthorized},
		"invalid token": {"Bearer bad-token", `"INVALID_TOKEN"`, "", http.StatusUnauthorized},
		"valid token":   {"bearer good-token", "", "user-1", http.StatusOK},
	} {
		handler, reachedAs := newProtectedTestHandler(authUseCase)
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		if tc.authorization != "" {
			req.Header.Set("Authorization", tc.authorization)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != tc.wantStatus || !strings.Contains(rec.Body.String(), tc.wantCode) {
			t.Fatalf("%s: status = %d body = %s, want %d %s", name, rec.Code, rec.Body.String(), tc.wantStatus, tc.wantCode)
		}
		if *reachedAs != tc.wantUser {
			t.Fatalf("%s: handler reached as %q, want %q", name, *reachedAs, tc.wantUser)
		}
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
