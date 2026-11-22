/**
##
## OverDrive 2026
## All Technical rights reserved
##
## healthcheck_test.go - Unit tests for the container health probe.
##
*/

package bootstrap

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// healthServer serves handler on 127.0.0.1 and returns its port.
func healthServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatalf("split %s: %v", server.URL, err)
	}
	return port
}

// TestHealthcheck_OK proves a 200 on GET /health is healthy and the probe hits that path.
func TestHealthcheck_OK(t *testing.T) {
	port := healthServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := Healthcheck(context.Background(), port); err != nil {
		t.Fatalf("Healthcheck: %v", err)
	}
}

// TestHealthcheck_NotOK proves any other status is unhealthy and named in the error.
func TestHealthcheck_NotOK(t *testing.T) {
	port := healthServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if err := Healthcheck(context.Background(), port); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err = %v, want the 503 status", err)
	}
}

// TestHealthcheck_Unreachable proves a port nothing listens on is unhealthy.
func TestHealthcheck_Unreachable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	_ = listener.Close()
	if err := Healthcheck(context.Background(), port); err == nil {
		t.Fatal("want an error for a closed port")
	}
}

// TestHealthcheck_InvalidPort proves the probe refuses a missing or non-numeric HTTP_PORT, or
// one out of range, before any request (the host is never taken from the environment).
func TestHealthcheck_InvalidPort(t *testing.T) {
	for _, port := range []string{"", "abc", "0", "65536", "80@evil.example", "1/x"} {
		if err := Healthcheck(context.Background(), port); err == nil || !strings.Contains(err.Error(), "HTTP_PORT") {
			t.Fatalf("port %q: err = %v, want the HTTP_PORT error", port, err)
		}
	}
}
