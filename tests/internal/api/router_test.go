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
	"bytes"
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

// TestRouterAccessLogStructured verifies access logs are emitted as structured JSON with nested request context.
func TestRouterAccessLogStructured(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "timestamp"
			case slog.MessageKey:
				attr.Key = "message"
			case slog.LevelKey:
				attr.Value = slog.StringValue(attr.Value.String())
			}
			return attr
		},
	}))

	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{}), api.RaceDefaults{}, time.Second),
		logger,
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "172.18.0.1:35842"
	req.Header.Set("User-Agent", "overdrive-test")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logBuffer.Bytes()), &payload); err != nil {
		t.Fatalf("failed to decode log payload: %v", err)
	}

	if payload["timestamp"] == nil {
		t.Fatalf("expected timestamp in log payload: %#v", payload)
	}
	if payload["message"] != "HTTP request completed" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
	if payload["type"] != "http_request" {
		t.Fatalf("unexpected type: %#v", payload["type"])
	}

	request, ok := payload["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested request object: %#v", payload)
	}
	if request["method"] != "GET" || request["path"] != "/health" {
		t.Fatalf("unexpected request payload: %#v", request)
	}
	if request["client_ip"] != "172.18.0.1" {
		t.Fatalf("unexpected client_ip: %#v", request["client_ip"])
	}
	if request["id"] == nil || request["id"] == "" {
		t.Fatalf("expected request id in log payload: %#v", request)
	}
}
