/**
##
## OverDrive 2026
## All Technical rights reserved
##
## router_test.go - Unit tests for router wiring and middleware behavior.
##
*/

package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"overdrive/internal/api"
	"overdrive/internal/domain"
	"overdrive/internal/service"
	"overdrive/tests/internal/mocks"
)

// TestRouterHealthAddsRequestID verifies middleware injects a request ID on routed responses.
func TestRouterHealthAddsRequestID(t *testing.T) {
	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{}), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
}

// TestRouterDriverProfileRoute verifies path-parameter routes are wired to the correct handler.
func TestRouterDriverProfileRoute(t *testing.T) {
	archive := mocks.SampleArchive()
	store := &mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	}

	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, store), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/drivers/1/profile", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if int(payload["driver_number"].(float64)) != 1 {
		t.Fatalf("unexpected driver number: %#v", payload["driver_number"])
	}
}
