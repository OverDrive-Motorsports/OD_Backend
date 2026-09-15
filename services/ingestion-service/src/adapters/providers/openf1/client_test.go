/**
##
## OverDrive 2026
## All Technical rights reserved
##
## client_test.go - Package openf1 source file for services/ingestion-service/src/adapters/providers/openf1.
##
*/

package openf1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
)

func intPtr(v int) *int { return &v }

// jsonHandler responds with the given status/body for every request, and lets tests intercept
// the request (path/query) via onRequest.
func jsonHandler(t *testing.T, status int, body any, onRequest func(r *http.Request)) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if onRequest != nil {
			onRequest(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

// TestClient_Fetch_Success proves Fetch decodes the upstream JSON array into the expected row
// shape for a plain (non-starting_grid) resource.
func TestClient_Fetch_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(t, http.StatusOK, []map[string]any{{"driver_number": 63}}, nil))
	defer server.Close()

	c := NewClient(server.URL, time.Second, 0, time.Millisecond)
	rows, err := c.Fetch(context.Background(), "drivers", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0]["driver_number"] != float64(63) {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

// TestClient_Fetch_UnsupportedResource proves Fetch surfaces domain.ErrUnsupportedResource
// (via buildURL) for a resource outside the allow-list, without making any HTTP request.
func TestClient_Fetch_UnsupportedResource(t *testing.T) {
	requested := false
	server := httptest.NewServer(jsonHandler(t, http.StatusOK, []map[string]any{}, func(r *http.Request) { requested = true }))
	defer server.Close()

	c := NewClient(server.URL, time.Second, 0, time.Millisecond)
	_, err := c.Fetch(context.Background(), "not-a-real-resource", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
	if !errors.Is(err, domain.ErrUnsupportedResource) {
		t.Fatalf("expected domain.ErrUnsupportedResource, got %v", err)
	}
	if requested {
		t.Fatal("expected no HTTP request for an unsupported resource")
	}
}

// TestClient_BuildURL proves query param construction per resource: meeting-only resources omit
// session_key, driver-scoped resources include driver_number only when supplied, and an
// unsupported resource returns the domain sentinel.
func TestClient_BuildURL(t *testing.T) {
	c := NewClient("https://api.openf1.org/v1/", time.Second, 0, time.Millisecond)

	t.Run("meetings has meeting_key only", func(t *testing.T) {
		endpoint, err := c.buildURL("meetings", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(endpoint, "meeting_key=1141") || strings.Contains(endpoint, "session_key") {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		if !strings.HasPrefix(endpoint, "https://api.openf1.org/v1/meetings?") {
			t.Fatalf("expected trailing slash on base URL to be trimmed, got %s", endpoint)
		}
	})

	t.Run("driver-scoped resource without driver number omits driver_number", func(t *testing.T) {
		endpoint, err := c.buildURL("car_data", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(endpoint, "driver_number") {
			t.Fatalf("expected no driver_number, got %s", endpoint)
		}
	})

	t.Run("driver-scoped resource with driver number includes it", func(t *testing.T) {
		endpoint, err := c.buildURL("car_data", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998, DriverNumber: intPtr(63)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(endpoint, "driver_number=63") {
			t.Fatalf("expected driver_number=63, got %s", endpoint)
		}
	})

	t.Run("championship_drivers includes driver_number when supplied", func(t *testing.T) {
		endpoint, err := c.buildURL("championship_drivers", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998, DriverNumber: intPtr(1)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(endpoint, "driver_number=1") || !strings.Contains(endpoint, "session_key=9998") {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
	})

	t.Run("unsupported resource", func(t *testing.T) {
		_, err := c.buildURL("not-a-real-resource", domain.OpenF1IngestionRequest{})
		if !errors.Is(err, domain.ErrUnsupportedResource) {
			t.Fatalf("expected domain.ErrUnsupportedResource, got %v", err)
		}
	})
}

// TestClient_FetchWithRetry_SucceedsFirstTry proves a successful first attempt returns
// immediately without invoking the retry/backoff path.
func TestClient_FetchWithRetry_SucceedsFirstTry(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(jsonHandler(t, http.StatusOK, []map[string]any{{"ok": true}}, func(r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
	}))
	defer server.Close()

	c := NewClient(server.URL, time.Second, 2, time.Millisecond)
	rows, err := c.fetchWithRetry(context.Background(), server.URL+"/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if got := atomic.LoadInt32(&requestCount); got != 1 {
		t.Fatalf("request count = %d, want 1 (no retry expected)", got)
	}
}

// TestClient_FetchWithRetry_SucceedsAfterRetry proves a transient failure is retried and the
// eventual success is returned, with the observed attempt count matching the failure count + 1.
func TestClient_FetchWithRetry_SucceedsAfterRetry(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&requestCount, 1)
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"ok": true}})
	}))
	defer server.Close()

	c := NewClient(server.URL, time.Second, 2, time.Millisecond)
	rows, err := c.fetchWithRetry(context.Background(), server.URL+"/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if got := atomic.LoadInt32(&requestCount); got != 3 {
		t.Fatalf("request count = %d, want 3 (2 failures + 1 success)", got)
	}
}

// TestClient_FetchWithRetry_ExhaustedSurfacesFinalError proves that once every retry is spent
// the last observed error is returned, and the number of attempts is bounded to retries+1.
func TestClient_FetchWithRetry_ExhaustedSurfacesFinalError(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	c := NewClient(server.URL, time.Second, 2, time.Millisecond)
	_, err := c.fetchWithRetry(context.Background(), server.URL+"/x")
	if err == nil {
		t.Fatal("expected an error once retries are exhausted")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected the final upstream error to be surfaced, got %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 3 {
		t.Fatalf("request count = %d, want 3 (retries=2 -> 1 initial + 2 retries)", got)
	}
}

// TestClient_FetchOnce_StatusHandling proves fetchOnce decodes a 2xx body, and distinguishes a
// 4xx ("request failed") from a 5xx ("upstream status") error message, both carrying the
// response body for diagnostics.
func TestClient_FetchOnce_StatusHandling(t *testing.T) {
	t.Run("2xx decodes rows", func(t *testing.T) {
		server := httptest.NewServer(jsonHandler(t, http.StatusOK, []map[string]any{{"a": 1}}, nil))
		defer server.Close()
		c := NewClient(server.URL, time.Second, 0, time.Millisecond)

		rows, err := c.fetchOnce(context.Background(), server.URL+"/x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("unexpected rows: %+v", rows)
		}
	})

	t.Run("4xx reports a request failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer server.Close()
		c := NewClient(server.URL, time.Second, 0, time.Millisecond)

		_, err := c.fetchOnce(context.Background(), server.URL+"/x")
		if err == nil || !strings.Contains(err.Error(), "status 404") {
			t.Fatalf("expected an error mentioning status 404, got %v", err)
		}
	})

	t.Run("5xx reports an upstream status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		}))
		defer server.Close()
		c := NewClient(server.URL, time.Second, 0, time.Millisecond)

		_, err := c.fetchOnce(context.Background(), server.URL+"/x")
		if err == nil || !strings.Contains(err.Error(), "upstream status 500") {
			t.Fatalf("expected an error mentioning upstream status 500, got %v", err)
		}
	})
}

// startingGridServer builds a fake OpenF1 server that fails the initial starting_grid request
// for sessionKey with the given status, and serves /sessions (meeting-scoped) from the provided
// sessions rows, and /starting_grid for the fallback session key with fallbackRows.
type startingGridServer struct {
	server             *httptest.Server
	startingGridHits   int32
	sessionsHits       int32
	fallbackGridHits   int32
	initialStatus      int
	sessions           []map[string]any
	fallbackSessionKey int
	fallbackRows       []map[string]any
}

func newStartingGridServer(t *testing.T, initialStatus int, sessions []map[string]any, fallbackSessionKey int, fallbackRows []map[string]any) *startingGridServer {
	t.Helper()
	s := &startingGridServer{
		initialStatus:      initialStatus,
		sessions:           sessions,
		fallbackSessionKey: fallbackSessionKey,
		fallbackRows:       fallbackRows,
	}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/sessions"):
			atomic.AddInt32(&s.sessionsHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(s.sessions)
		case strings.HasPrefix(r.URL.Path, "/starting_grid"):
			sessionKey := r.URL.Query().Get("session_key")
			if sessionKey == strconv.Itoa(s.fallbackSessionKey) && s.fallbackSessionKey != 0 {
				atomic.AddInt32(&s.fallbackGridHits, 1)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(s.fallbackRows)
				return
			}
			atomic.AddInt32(&s.startingGridHits, 1)
			w.WriteHeader(s.initialStatus)
			_, _ = w.Write([]byte("not found"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return s
}

// TestClient_FetchStartingGrid_DirectSuccess proves a Race session whose starting_grid endpoint
// responds directly is returned as-is, with no fallback lookup performed.
func TestClient_FetchStartingGrid_DirectSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/sessions") {
			t.Fatal("expected no fallback sessions lookup when the direct request succeeds")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"driver_number": 1, "position": 1}})
	}))
	defer server.Close()

	c := NewClient(server.URL, time.Second, 0, time.Millisecond)
	rows, err := c.Fetch(context.Background(), "starting_grid", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

// TestClient_FetchStartingGrid_NonNotFoundErrorIsNotFallenBack proves that a non-404 failure
// (e.g. a 500) on the direct starting_grid request is returned as-is, without attempting the
// qualifying-session fallback lookup - only a 404 triggers the fallback per README.
func TestClient_FetchStartingGrid_NonNotFoundErrorIsNotFallenBack(t *testing.T) {
	sessionsHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/sessions") {
			sessionsHit = true
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	c := NewClient(server.URL, time.Second, 0, time.Millisecond)
	_, err := c.Fetch(context.Background(), "starting_grid", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
	if err == nil {
		t.Fatal("expected an error")
	}
	if sessionsHit {
		t.Fatal("expected no fallback lookup for a non-404 error")
	}
}

// TestClient_FetchStartingGrid_FallsBackToPrecedingQualifying proves the README-documented
// fallback: a Race session with a 404 starting_grid finds the preceding non-sprint Qualifying
// session in the same meeting, fetches its grid, and annotates the fallback rows with the
// original session/meeting key plus source_session_key so downstream consumers can trace it.
func TestClient_FetchStartingGrid_FallsBackToPrecedingQualifying(t *testing.T) {
	sessions := []map[string]any{
		{"session_key": 9990, "session_name": "Practice 1", "session_type": "Practice"},
		{"session_key": 9995, "session_name": "Qualifying", "session_type": "Qualifying"},
		{"session_key": 9998, "session_name": "Race", "session_type": "Race"},
	}
	fallbackRows := []map[string]any{{"driver_number": 1, "position": 1}}
	fake := newStartingGridServer(t, http.StatusNotFound, sessions, 9995, fallbackRows)
	defer fake.server.Close()

	c := NewClient(fake.server.URL, time.Second, 0, time.Millisecond)
	rows, err := c.Fetch(context.Background(), "starting_grid", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if rows[0]["session_key"] != 9998 || rows[0]["meeting_key"] != 1141 || rows[0]["source_session_key"] != 9995 {
		t.Fatalf("expected fallback rows annotated with the original session/meeting and source_session_key, got %+v", rows[0])
	}
	if atomic.LoadInt32(&fake.sessionsHits) != 1 {
		t.Fatalf("expected exactly one sessions lookup, got %d", fake.sessionsHits)
	}
}

// TestClient_FetchStartingGrid_SprintFallsBackToSprintQualifying proves the sprint-specific
// branch: a Sprint session's fallback prefers the preceding Sprint Qualifying session over a
// plain Qualifying session, per README.
func TestClient_FetchStartingGrid_SprintFallsBackToSprintQualifying(t *testing.T) {
	sessions := []map[string]any{
		{"session_key": 9990, "session_name": "Qualifying", "session_type": "Qualifying"},
		{"session_key": 9993, "session_name": "Sprint Qualifying", "session_type": "Qualifying"},
		{"session_key": 9998, "session_name": "Sprint", "session_type": "Race"},
	}
	fallbackRows := []map[string]any{{"driver_number": 1, "position": 1}}
	fake := newStartingGridServer(t, http.StatusNotFound, sessions, 9993, fallbackRows)
	defer fake.server.Close()

	c := NewClient(fake.server.URL, time.Second, 0, time.Millisecond)
	rows, err := c.Fetch(context.Background(), "starting_grid", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0]["source_session_key"] != 9993 {
		t.Fatalf("expected the sprint qualifying session to be used as the fallback source, got %+v", rows)
	}
}

// TestClient_FetchStartingGrid_NoQualifyingFoundReturnsWrappedError proves that when no
// qualifying session precedes the target session, the original 404 error and the lookup failure
// are both surfaced (wrapped together) rather than one silently replacing the other.
func TestClient_FetchStartingGrid_NoQualifyingFoundReturnsWrappedError(t *testing.T) {
	sessions := []map[string]any{
		{"session_key": 9998, "session_name": "Race", "session_type": "Race"},
	}
	fake := newStartingGridServer(t, http.StatusNotFound, sessions, 0, nil)
	defer fake.server.Close()

	c := NewClient(fake.server.URL, time.Second, 0, time.Millisecond)
	_, err := c.Fetch(context.Background(), "starting_grid", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "status 404") || !strings.Contains(err.Error(), "fallback lookup failed") {
		t.Fatalf("expected the wrapped original+fallback error, got %v", err)
	}
}

// TestClient_FetchStartingGrid_SessionNotFoundInMeeting proves that if the requested session key
// itself isn't present among the meeting's sessions, findStartingGridSession fails clearly
// instead of silently returning a wrong session.
func TestClient_FetchStartingGrid_SessionNotFoundInMeeting(t *testing.T) {
	sessions := []map[string]any{
		{"session_key": 1234, "session_name": "Practice 1", "session_type": "Practice"},
	}
	fake := newStartingGridServer(t, http.StatusNotFound, sessions, 0, nil)
	defer fake.server.Close()

	c := NewClient(fake.server.URL, time.Second, 0, time.Millisecond)
	_, err := c.Fetch(context.Background(), "starting_grid", domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
	if err == nil || !strings.Contains(err.Error(), "fallback lookup failed") {
		t.Fatalf("expected a wrapped fallback lookup error, got %v", err)
	}
}

// TestIntValue proves intValue coerces every OpenF1 numeric payload shape (JSON-decoded
// float64, plain int variants, numeric strings) to an int, and defaults to 0 for anything else
// (nil, an unparsable string, or an unrelated type) rather than panicking.
func TestIntValue(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{"int", int(7), 7},
		{"int32", int32(7), 7},
		{"int64", int64(7), 7},
		{"float64", float64(7), 7},
		{"numeric string", "42", 42},
		{"unparsable string", "not-a-number", 0},
		{"nil", nil, 0},
		{"unrelated type", true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := intValue(tc.value); got != tc.want {
				t.Fatalf("intValue(%v) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}
