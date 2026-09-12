/**
##
## OverDrive 2026
## All Technical rights reserved
##
## middleware_test.go - Package httpadapter source file for services/ingestion-service/src/adapters/http.
##
*/

package httpadapter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWithSecurityHeaders proves the baseline hardening headers are applied to every response,
// and that the wrapped handler still runs.
func TestWithSecurityHeaders(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := withSecurityHeaders(next)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected the wrapped handler to be invoked")
	}
	wantHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
	}
	for key, want := range wantHeaders {
		if got := rec.Header().Get(key); got != want {
			t.Fatalf("header %q = %q, want %q", key, got, want)
		}
	}
}

// TestLimitRequestBody proves a request body within the limit is passed through untouched, and a
// body exceeding the limit is rejected before it reaches application code.
func TestLimitRequestBody(t *testing.T) {
	t.Run("within limit passes through", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("unexpected read error: %v", err)
			}
			if string(body) != "ok" {
				t.Fatalf("body = %q, want ok", string(body))
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := limitRequestBody(next, 10)
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("ok"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("over limit is rejected", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := io.ReadAll(r.Body); err == nil {
				t.Fatal("expected reading an over-limit body to fail")
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := limitRequestBody(next, 4)
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("way too long"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
	})
}
