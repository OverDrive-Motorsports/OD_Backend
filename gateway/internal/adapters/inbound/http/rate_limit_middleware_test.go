/**
##
## OverDrive 2026
## All Technical rights reserved
##
## rate_limit_middleware_test.go - Unit tests for the token-bucket rate limiter.
##
*/

package httpinbound

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLimiter_AllowsUpToBurstThenBlocks(t *testing.T) {
	l := newLimiter(1, 3)

	for i := 0; i < 3; i++ {
		if !l.allow("client-a") {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}

	if l.allow("client-a") {
		t.Fatal("request exceeding burst should be blocked")
	}
}

func TestLimiter_TracksClientsIndependently(t *testing.T) {
	l := newLimiter(1, 1)

	if !l.allow("client-a") {
		t.Fatal("first request for client-a should be allowed")
	}
	if l.allow("client-a") {
		t.Fatal("second immediate request for client-a should be blocked")
	}
	if !l.allow("client-b") {
		t.Fatal("client-b should have its own independent bucket")
	}
}

func TestRateLimit_ReturnsTooManyRequestsWhenExceeded(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(1, 1, next)

	req1 := httptest.NewRequest(http.MethodGet, "/v1/some-route", nil)
	req1.RemoteAddr = "192.0.2.1:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/some-route", nil)
	req2.RemoteAddr = "192.0.2.1:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{"uses X-Forwarded-For first hop", "192.0.2.1:1234", "203.0.113.5, 198.51.100.9", "203.0.113.5"},
		{"falls back to remote addr host", "192.0.2.1:1234", "", "192.0.2.1"},
		{"handles remote addr without port", "192.0.2.1", "", "192.0.2.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/some-route", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}

			if got := getClientIP(req); got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
