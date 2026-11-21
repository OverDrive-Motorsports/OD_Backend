/**
##
## OverDrive 2026
## All Technical rights reserved
##
## logging_middleware_test.go - Unit tests for request logging.
##
*/

package httpinbound

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// captureLog redirects the standard logger to a buffer for the duration of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previousOutput, previousFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})
	return &buf
}

// TestRequestLogger_QuotesPathSoItCannotForgeLogLines proves that a decoded
// newline in the request path (sent as %0A) stays escaped inside one log line
// instead of starting a forged one.
func TestRequestLogger_QuotesPathSoItCannotForgeLogLines(t *testing.T) {
	buf := captureLog(t)
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/x%0AGET%20/admin%20status=200", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	got := strings.TrimSuffix(buf.String(), "\n")
	if strings.Contains(got, "\n") {
		t.Fatalf("log output spans several lines: %q", got)
	}
	if !strings.HasPrefix(got, `"GET" "/v1/x\nGET /admin status=200" status=204 `) {
		t.Fatalf("unexpected log line: %q", got)
	}
}

// TestRequestLogger_LogsPlainPathQuoted proves the normal line format: quoted
// method, quoted path, recorded status and quoted remote address.
func TestRequestLogger_LogsPlainPathQuoted(t *testing.T) {
	buf := captureLog(t)
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/race-data/sessions", nil))

	if !strings.HasPrefix(buf.String(), `"POST" "/v1/race-data/sessions" status=418 `) {
		t.Fatalf("unexpected log line: %q", buf.String())
	}
	if !strings.HasSuffix(buf.String(), ` remote="192.0.2.1:1234"`+"\n") {
		t.Fatalf("remote address not quoted: %q", buf.String())
	}
}

// TestReverseProxy_QuotesPathInErrorLog proves that the upstream error log of
// the reverse proxy escapes a newline in the path as well.
func TestReverseProxy_QuotesPathInErrorLog(t *testing.T) {
	buf := captureLog(t)
	deadURL, err := url.Parse("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	proxy, err := NewReverseProxy("/v1/race-data", []*url.URL{deadURL})
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	proxy.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/race-data/a%0Ab", nil))

	got := strings.TrimSuffix(buf.String(), "\n")
	if strings.Contains(got, "\n") {
		t.Fatalf("log output spans several lines: %q", got)
	}
	if !strings.Contains(got, `proxy error for "/a\nb"`) {
		t.Fatalf("unexpected log line: %q", got)
	}
}
