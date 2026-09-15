/**
##
## OverDrive 2026
## All Technical rights reserved
##
## middleware_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
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

// TestWithSecurityHeaders proves every documented baseline hardening header is set on the
// response and that the wrapped handler still runs.
func TestWithSecurityHeaders(t *testing.T) {
	called := false
	handler := withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected the wrapped handler to run")
	}
	headers := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
	}
	for name, want := range headers {
		if got := rec.Header().Get(name); got != want {
			t.Fatalf("header %s = %q, want %q", name, got, want)
		}
	}
}

// TestLimitRequestBody proves a body larger than the configured limit is rejected when the handler
// tries to read past the limit, and a body within the limit reads through unaffected.
func TestLimitRequestBody(t *testing.T) {
	const limit = 8

	handler := limitRequestBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}), limit)

	t.Run("within limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("short"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("over limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("this body is definitely over the limit"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", rec.Code)
		}
	})
}
