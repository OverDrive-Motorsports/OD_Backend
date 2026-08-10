/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## middleware_test.go - Package httpadapter source file for services/championship-service/src/adapters/http.
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

// TestWithSecurityHeaders proves the baseline hardening headers are set on every response and
// the wrapped handler still runs.
func TestWithSecurityHeaders(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	withSecurityHeaders(inner).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected the inner handler to run")
	}
	wantHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
	}
	for header, want := range wantHeaders {
		if got := rec.Header().Get(header); got != want {
			t.Fatalf("header %q = %q, want %q", header, got, want)
		}
	}
}

// TestLimitRequestBody proves a body within the limit is read in full, and a body over the
// limit fails to read past the cap rather than being silently truncated and accepted.
func TestLimitRequestBody(t *testing.T) {
	const limit = 8

	t.Run("body within the limit is read in full", func(t *testing.T) {
		var readErr error
		var readBody string
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			readErr = err
			readBody = string(body)
		})

		req := httptest.NewRequest(http.MethodPost, "/ingestion", strings.NewReader("1234567"))
		rec := httptest.NewRecorder()

		limitRequestBody(inner, limit).ServeHTTP(rec, req)

		if readErr != nil {
			t.Fatalf("unexpected read error: %v", readErr)
		}
		if readBody != "1234567" {
			t.Fatalf("got %q, want %q", readBody, "1234567")
		}
	})

	t.Run("body over the limit fails to read fully", func(t *testing.T) {
		var readErr error
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, readErr = io.ReadAll(r.Body)
		})

		req := httptest.NewRequest(http.MethodPost, "/ingestion", strings.NewReader("123456789"))
		rec := httptest.NewRecorder()

		limitRequestBody(inner, limit).ServeHTTP(rec, req)

		if readErr == nil {
			t.Fatal("expected a body-too-large error, got nil")
		}
	})
}
