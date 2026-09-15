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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuthorization(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		header     string
		authorize  func(string) bool
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "missing header is rejected",
			header:     "",
			authorize:  func(string) bool { return true },
			wantStatus: http.StatusUnauthorized,
			wantNext:   false,
		},
		{
			name:       "invalid token is rejected",
			header:     "Bearer bad-token",
			authorize:  func(string) bool { return false },
			wantStatus: http.StatusUnauthorized,
			wantNext:   false,
		},
		{
			name:       "valid token passes through",
			header:     "Bearer good-token",
			authorize:  func(string) bool { return true },
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false
			middleware := RequireAuthorization(tt.authorize, next)

			req := httptest.NewRequest(http.MethodGet, "/v1/some-route", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if nextCalled != tt.wantNext {
				t.Errorf("next called = %v, want %v", nextCalled, tt.wantNext)
			}
		})
	}
}
