/**
##
## OverDrive 2026
## All Technical rights reserved
##
## auth_middleware_test.go - Unit tests for RequireAuthorization.
##
*/

package httpinbound

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/gateway/internal/core/domain"
)

// TestRequireUserToken proves the middleware calls the next handler only when authorize accepts
// the header, forwards the header unchanged to authorize, and maps each rejection reason to its
// own status and code: UNAUTHORIZED, TOKEN_EXPIRED and INVALID_TOKEN (401), FORBIDDEN (403).
// An unknown reason is treated as an invalid token (fail closed).
func TestRequireUserToken(t *testing.T) {
	tests := []struct {
		name       string
		reason     error
		wantStatus int
		wantCode   string
	}{
		{"accepted", nil, http.StatusOK, ""},
		{"missing token", domain.ErrMissingUserToken, http.StatusUnauthorized, `"UNAUTHORIZED"`},
		{"expired token", domain.ErrUserTokenExpired, http.StatusUnauthorized, `"TOKEN_EXPIRED"`},
		{"invalid token", domain.ErrInvalidUserToken, http.StatusUnauthorized, `"INVALID_TOKEN"`},
		{"role not allowed", domain.ErrForbiddenRole, http.StatusForbidden, `"FORBIDDEN"`},
		{"unknown reason", errors.New("boom"), http.StatusUnauthorized, `"INVALID_TOKEN"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})
			var gotHeader string
			authorize := func(header string) error {
				gotHeader = header
				return tt.reason
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/some-route", nil)
			req.Header.Set("Authorization", "Bearer abc")
			rec := httptest.NewRecorder()
			RequireUserToken(authorize, next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantCode) {
				t.Fatalf("status = %d body = %s, want %d %s", rec.Code, rec.Body.String(), tt.wantStatus, tt.wantCode)
			}
			if nextCalled != (tt.reason == nil) {
				t.Fatalf("next called = %v, want %v", nextCalled, tt.reason == nil)
			}
			if gotHeader != "Bearer abc" {
				t.Fatalf("authorize got header %q", gotHeader)
			}
		})
	}
}
