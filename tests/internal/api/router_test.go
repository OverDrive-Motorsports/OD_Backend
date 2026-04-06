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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"overdrive/internal/api"
	"overdrive/internal/config"
	"overdrive/internal/domain"
	"overdrive/internal/service"
	"overdrive/tests/internal/mocks"
)

// TestRouterHealthAddsRequestID verifies middleware injects a request ID on routed responses.
func TestRouterHealthAddsRequestID(t *testing.T) {
	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{}), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		config.Config{},
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
		config.Config{},
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
		config.Config{},
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

func TestRouterCORSPreflightAllowed(t *testing.T) {
	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{}), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		config.Config{CORSAllowedOrigins: []string{"http://localhost:3000"}},
	)

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("unexpected allow origin: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRouterCORSRejectsUnknownOrigin(t *testing.T) {
	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{}), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		config.Config{CORSAllowedOrigins: []string{"http://localhost:3000"}},
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRouterRateLimitRejectsBurst(t *testing.T) {
	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{}), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		config.Config{
			RateLimitRequests: 1,
			RateLimitWindow:   time.Minute,
		},
	)

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "172.18.0.1:40000"
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("unexpected first status: %d body=%s", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "172.18.0.1:40000"
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected second status: %d body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestRouterRejectsInvalidJWT(t *testing.T) {
	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{}), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		config.Config{JWTSecret: "top-secret"},
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRouterAcceptsValidJWT(t *testing.T) {
	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{}), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		config.Config{
			JWTSecret:   "top-secret",
			JWTIssuer:   "overdrive",
			JWTAudience: "beta-client",
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer "+signedTestJWT(t, "top-secret"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func signedTestJWT(t *testing.T, secret string) string {
	t.Helper()

	header := `{"alg":"HS256","typ":"JWT"}`
	claims := fmt.Sprintf(`{"sub":"user-1","user_id":"user-1","exp":%d,"iss":"overdrive","aud":"beta-client","roles":["beta_tester"]}`, time.Now().Add(time.Hour).Unix())

	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encodedClaims := base64.RawURLEncoding.EncodeToString([]byte(claims))
	signingInput := encodedHeader + "." + encodedClaims

	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(signingInput))
	if err != nil {
		t.Fatalf("failed to sign jwt: %v", err)
	}
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + signature
}
