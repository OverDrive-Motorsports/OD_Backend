/**
##
## OverDrive 2026
## All Technical rights reserved
##
## client_test.go - Package championshipclient source file for services/race-data-service/src/adapters/downstream/championship.
##
*/

package championshipclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(server.URL, 2*time.Second)
}

// TestClient_GetSession_SuccessDecode proves a 200 JSON response decodes into
// ports.ChampionshipSessionRef correctly.
func TestClient_GetSession_SuccessDecode(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/s1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"sessionId": "s1", "name": "Race"})
	})

	got, err := client.GetSession(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.ID != "s1" || got.Name != "Race" {
		t.Fatalf("unexpected decoded session: %+v", got)
	}
}

// TestClient_GetSession_UpstreamNotFound proves a 404 upstream response translates into (nil, nil)
// rather than an error - this is how the usecase layer's "session doesn't exist" contract is built.
func TestClient_GetSession_UpstreamNotFound(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	got, err := client.GetSession(context.Background(), "missing")
	if err != nil || got != nil {
		t.Fatalf("expected nil, nil for a 404 upstream, got %v %v", got, err)
	}
}

// TestClient_GetSession_UpstreamServerError proves a 5xx (or any >=400 non-404) upstream response
// is surfaced as a real error, distinct from the 404 "not found" case.
func TestClient_GetSession_UpstreamServerError(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	got, err := client.GetSession(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected an error for a 500 upstream response")
	}
	if got != nil {
		t.Fatalf("expected a nil payload alongside the error, got %+v", got)
	}
}

// TestClient_GetSession_MalformedJSON proves a response body that isn't valid JSON is surfaced as
// a decode error.
func TestClient_GetSession_MalformedJSON(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	})

	_, err := client.GetSession(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected a JSON decode error")
	}
}

// TestClient_GetSession_ConnectionFailure proves a request to an address with nothing listening
// surfaces as an error rather than hanging or panicking.
func TestClient_GetSession_ConnectionFailure(t *testing.T) {
	client := New("http://127.0.0.1:1", 200*time.Millisecond)

	_, err := client.GetSession(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected a connection error")
	}
}

// TestClient_ListChampionships_BareArrayPassThrough proves the collection methods (which
// championship-service now returns as a bare JSON array) decode and forward that array unchanged.
func TestClient_ListChampionships_BareArrayPassThrough(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/championships" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"code": "f1"}, {"code": "f2"}})
	})

	got, err := client.ListChampionships(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items, ok := got.([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected a 2-element bare array, got %#v", got)
	}
}

// TestClient_ListChampionshipEvents_PathBuilding proves the code parameter is interpolated into
// the request path correctly.
func TestClient_ListChampionshipEvents_PathBuilding(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/championships/f1/events" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	if _, err := client.ListChampionshipEvents(context.Background(), "f1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_GetEventPayload_And_GetEvent proves both event-fetching methods hit the same
// /events/{id} path but decode into different shapes (any vs *ports.ChampionshipEventRef).
func TestClient_GetEventPayload_And_GetEvent(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/e1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"eventId": "e1", "name": "Monaco GP", "seasonYear": 2026})
	})

	payload, err := client.GetEventPayload(context.Background(), "e1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m, ok := payload.(map[string]any); !ok || m["name"] != "Monaco GP" {
		t.Fatalf("unexpected event payload: %#v", payload)
	}

	event, err := client.GetEvent(context.Background(), "e1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil || event.Name != "Monaco GP" || event.SeasonYear != 2026 {
		t.Fatalf("unexpected decoded event: %+v", event)
	}
}

// TestClient_ListEventSessions_PathBuilding proves the eventID parameter is interpolated into the
// request path correctly.
func TestClient_ListEventSessions_PathBuilding(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/e1/sessions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	if _, err := client.ListEventSessions(context.Background(), "e1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_ListSessionDrivers_DecodesCamelCaseFields proves the bare-array driver list decodes
// into ports.ChampionshipDriverRef using its camelCase JSON tags.
func TestClient_ListSessionDrivers_DecodesCamelCaseFields(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/s1/drivers" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"driverNumber": 44, "fullName": "Lewis Hamilton"}})
	})

	got, err := client.ListSessionDrivers(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].DriverNumber != 44 || got[0].DriverName != "Lewis Hamilton" {
		t.Fatalf("unexpected decoded drivers: %+v", got)
	}
}

// TestClient_ListSessionTeams_PathBuilding proves the sessionID parameter is interpolated into the
// request path correctly.
func TestClient_ListSessionTeams_PathBuilding(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/s1/teams" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	if _, err := client.ListSessionTeams(context.Background(), "s1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_GetSessionDataset_PathBuilding proves both the sessionID and dataset parameters are
// interpolated into the request path correctly.
func TestClient_GetSessionDataset_PathBuilding(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/s1/datasets/starting_grid" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"dataset": "starting_grid"})
	})
	got, err := client.GetSessionDataset(context.Background(), "s1", "starting_grid")
	if err != nil || got["dataset"] != "starting_grid" {
		t.Fatalf("unexpected result: %v %v", got, err)
	}
}

// TestClient_GetSessionRaceStandings_PathBuilding proves the request hits the /standings/race path.
func TestClient_GetSessionRaceStandings_PathBuilding(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/s1/standings/race" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"leader": "VER"})
	})
	got, err := client.GetSessionRaceStandings(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m, ok := got.(map[string]any); !ok || m["leader"] != "VER" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

// TestClient_GetSessionBroadcast_PathBuilding proves the request hits the /broadcast path.
func TestClient_GetSessionBroadcast_PathBuilding(t *testing.T) {
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/s1/broadcast" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"broadcastUrl": "https://x"})
	})
	got, err := client.GetSessionBroadcast(context.Background(), "s1")
	if err != nil || got["broadcastUrl"] != "https://x" {
		t.Fatalf("unexpected result: %v %v", got, err)
	}
}

// TestNew_TrimsTrailingSlash proves New normalizes a trailing slash on baseURL so path
// concatenation never produces a double slash.
func TestNew_TrimsTrailingSlash(t *testing.T) {
	client := New("http://example.com/", time.Second)
	if client.baseURL != "http://example.com" {
		t.Fatalf("expected trailing slash to be trimmed, got %q", client.baseURL)
	}
}

// TestClient_ListChampionshipSessions proves the call hits /championships/{code}/sessions with
// the code path-escaped, decodes the typed rows, maps 404 to (nil, nil), an empty body array
// to a non-nil empty slice, and any other error status to an error.
func TestClient_ListChampionshipSessions(t *testing.T) {
	var gotPath string
	client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`[{"sessionId":"s1","name":"Race","type":"race","startTime":"2024-03-02T15:00:00Z","eventName":"Bahrain Grand Prix","seasonYear":2024,"circuit":"Bahrain International Circuit"}]`))
	})
	got, err := client.ListChampionshipSessions(context.Background(), "f1/../x")
	if err != nil || len(got) != 1 || got[0].SessionID != "s1" || got[0].SeasonYear != 2024 || got[0].EventName != "Bahrain Grand Prix" || !got[0].StartTime.Equal(time.Date(2024, 3, 2, 15, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected result: %+v err=%v", got, err)
	}
	if gotPath != "/championships/f1%2F..%2Fx/sessions" {
		t.Fatalf("expected the code path-escaped, got %q", gotPath)
	}

	notFound := testServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	if got, err := notFound.ListChampionshipSessions(context.Background(), "nope"); err != nil || got != nil {
		t.Fatalf("expected (nil, nil) on 404, got %+v err=%v", got, err)
	}

	empty := testServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	if got, err := empty.ListChampionshipSessions(context.Background(), "f1"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("expected an empty non-nil slice, got %#v err=%v", got, err)
	}

	failing := testServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	if _, err := failing.ListChampionshipSessions(context.Background(), "f1"); err == nil {
		t.Fatal("expected an error on a 500 response")
	}
}
