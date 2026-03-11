/**
##
## OverDrive 2026
## All Technical rights reserved
##
## server_test.go - Unit tests for runtime HTTP server wiring.
##
*/

package app_test

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"overdrive/internal/api"
	"overdrive/internal/app"
	"overdrive/internal/config"
)

// TestNewHTTPServerWiresRuntimeDependencies verifies the HTTP server is created with expected runtime settings.
func TestNewHTTPServerWiresRuntimeDependencies(t *testing.T) {
	server := app.NewHTTPServer(
		config.Config{
			OpenF1BaseURL:   "https://api.openf1.org/v1",
			HTTPTimeout:     5 * time.Second,
			RequestInterval: 10 * time.Millisecond,
			MaxRetries:      1,
			RetryDelay:      10 * time.Millisecond,
			GetRaceTimeout:  time.Minute,
		},
		api.RaceDefaults{
			Year:        2025,
			CountryName: "Australia",
			MeetingName: "Australian Grand Prix",
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		":8080",
	)

	if server == nil {
		t.Fatal("expected server to be created")
	}
	if server.Addr != ":8080" {
		t.Fatalf("unexpected server addr: %s", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("expected handler to be wired")
	}
	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("unexpected read header timeout: %s", server.ReadHeaderTimeout)
	}

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	rec := newStatusRecorder()
	server.Handler.ServeHTTP(rec, req)
	if rec.status != http.StatusOK {
		t.Fatalf("unexpected health status: %d", rec.status)
	}
}

type statusRecorder struct {
	header http.Header
	status int
}

func newStatusRecorder() *statusRecorder {
	return &statusRecorder{header: make(http.Header), status: http.StatusOK}
}

// Header returns the response headers map.
func (r *statusRecorder) Header() http.Header {
	return r.header
}

// Write stores the response body length and keeps the default status when needed.
func (r *statusRecorder) Write(p []byte) (int, error) {
	return len(p), nil
}

// WriteHeader stores the last HTTP status code.
func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}
