/**
##
## OverDrive 2026
## All Technical rights reserved
##
## body_limit_test.go - Tests for the request body size limit of the public auth routes.
##
*/

package httpadapter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAuthRoutes_OversizedBodyIs413 proves every body-reading auth route refuses a body over
// authBodyLimitBytes with 413 PAYLOAD_TOO_LARGE through the real router, before the usecase
// runs (its fake errors would otherwise produce 401/500).
func TestAuthRoutes_OversizedBodyIs413(t *testing.T) {
	usecase := &fakeAuthUseCase{
		loginErr:    errors.New("must not be called"),
		registerErr: errors.New("must not be called"),
		refreshErr:  errors.New("must not be called"),
	}
	router := NewRouter(NewHealthController(nil), NewSessionController(nil), NewAuthController(usecase), usecase)
	oversized := `{"email":"` + strings.Repeat("a", int(authBodyLimitBytes)) + `"}`

	for _, path := range []string{"/login", "/register", "/refresh", "/oauth/google/login", "/oauth/google/register"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(oversized))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413 (body=%s)", rec.Code, rec.Body.String())
			}
			if env := decodeErrorEnvelope(t, rec.Body.Bytes()); env.Error.Code != "PAYLOAD_TOO_LARGE" {
				t.Fatalf("code = %q, want PAYLOAD_TOO_LARGE", env.Error.Code)
			}
		})
	}
}

// TestAuthRoutes_BodyUnderLimitReachesUsecase proves a normal-sized body still reaches the
// usecase (the limit does not reject legitimate requests).
func TestAuthRoutes_BodyUnderLimitReachesUsecase(t *testing.T) {
	usecase := &fakeAuthUseCase{loginErr: errors.New("reached")}
	router := NewRouter(NewHealthController(nil), NewSessionController(nil), NewAuthController(usecase), usecase)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"a@b.com","password":"pw"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 from the fake usecase error", rec.Code)
	}
}
