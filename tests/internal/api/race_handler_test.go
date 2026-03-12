/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_handler_test.go - Unit tests for race HTTP handlers using mocked storage.
##
*/

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"overdrive/internal/api"
	"overdrive/internal/domain"
	"overdrive/internal/service"
	"overdrive/tests/internal/mocks"
)

// TestHandleListChampionships verifies the championship catalog endpoint payload.
func TestHandleListChampionships(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		ListChampionshipsFn: func(ctx context.Context) ([]domain.ChampionshipSummary, error) {
			return []domain.ChampionshipSummary{mocks.SampleChampionship()}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/championships", nil)
	rec := httptest.NewRecorder()
	handler.HandleListChampionships(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if int(payload["count"].(float64)) != 1 {
		t.Fatalf("unexpected count: %#v", payload["count"])
	}
}

// TestHandleRoot verifies the root endpoint exposes service metadata and route listings.
func TestHandleRoot(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.HandleRoot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if payload["service"] != "overdrive-backend" {
		t.Fatalf("unexpected service: %#v", payload["service"])
	}

	routes := payload["routes"].([]any)
	if len(routes) == 0 {
		t.Fatal("expected route list to be populated")
	}
}

// TestHandleStorageStatusFound verifies storage status returns stored metadata when an archive exists.
func TestHandleStorageStatusFound(t *testing.T) {
	archive := mocks.SampleArchive()
	storedAt := archive.Metadata.GeneratedAt
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, storedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/storage", nil)
	rec := httptest.NewRecorder()
	handler.HandleStorageStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if payload["stored"] != true {
		t.Fatalf("expected stored=true, got %#v", payload["stored"])
	}
}

// TestHandleSendRaceNotFound verifies the full payload endpoint returns 404 when storage is empty.
func TestHandleSendRaceNotFound(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/race/sendrace", nil)
	rec := httptest.NewRecorder()
	handler.HandleSendRace(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorContains(t, rec, "no race data stored yet")
}

// TestHandleSendMetadata verifies the metadata endpoint returns meeting and session context.
func TestHandleSendMetadata(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/metadata", nil)
	rec := httptest.NewRecorder()
	handler.HandleSendMetadata(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if payload["meeting"] == nil || payload["race_session"] == nil {
		t.Fatalf("expected meeting and race_session in payload: %#v", payload)
	}
}

// TestHandleListDatasets verifies only public datasets are exposed with counts.
func TestHandleListDatasets(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/datasets", nil)
	rec := httptest.NewRecorder()
	handler.HandleListDatasets(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if int(payload["count"].(float64)) == 0 {
		t.Fatalf("expected dataset catalog entries: %#v", payload)
	}
}

// TestHandleSendDatasetConstructorsAlias verifies public aliases resolve to stored dataset names.
func TestHandleSendDatasetConstructorsAlias(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/datasets/constructors", nil)
	req.SetPathValue("dataset", "constructors")
	rec := httptest.NewRecorder()
	handler.HandleSendDataset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if payload["dataset"] != "championship_teams" {
		t.Fatalf("unexpected dataset alias resolution: %#v", payload["dataset"])
	}
}

// TestHandleSendDatasetAddsDriverName verifies generic dataset responses enrich rows with driver_name.
func TestHandleSendDatasetAddsDriverName(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/datasets/session_result", nil)
	req.SetPathValue("dataset", "session_result")
	rec := httptest.NewRecorder()
	handler.HandleSendDataset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	data := payload["data"].([]any)
	first := data[0].(map[string]any)
	if first["driver_name"] == nil || first["driver_name"] == "" {
		t.Fatalf("expected driver_name to be present: %#v", first)
	}
	if first["team_name"] == nil || first["team_name"] == "" {
		t.Fatalf("expected team_name to be present: %#v", first)
	}
}

// TestHandleSendDatasetUnknown verifies invalid dataset aliases return HTTP 400.
func TestHandleSendDatasetUnknown(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/datasets/unknown", nil)
	req.SetPathValue("dataset", "unknown")
	rec := httptest.NewRecorder()
	handler.HandleSendDataset(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	assertErrorContains(t, rec, "unknown dataset")
}

// TestHandleListDrivers verifies drivers are returned in sorted driver number order.
func TestHandleListDrivers(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/drivers", nil)
	rec := httptest.NewRecorder()
	handler.HandleListDrivers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	data := payload["data"].([]any)
	first := data[0].(map[string]any)
	if int(first["driver_number"].(float64)) != 1 {
		t.Fatalf("expected first driver_number to be 1, got %#v", first["driver_number"])
	}
}

// TestHandleListTeams verifies teams are derived from the driver roster without duplicates.
func TestHandleListTeams(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/teams", nil)
	rec := httptest.NewRecorder()
	handler.HandleListTeams(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if int(payload["count"].(float64)) != 2 {
		t.Fatalf("unexpected teams count: %#v", payload["count"])
	}
}

// TestHandleSendDriverProfile verifies the driver profile endpoint returns only the requested driver.
func TestHandleSendDriverProfile(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/drivers/63/profile", nil)
	req.SetPathValue("driverNumber", "63")
	rec := httptest.NewRecorder()
	handler.HandleSendDriverProfile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if int(payload["driver_number"].(float64)) != 63 {
		t.Fatalf("unexpected driver_number: %#v", payload["driver_number"])
	}
	driver := payload["driver"].(map[string]any)
	if driver["full_name"] != "George Russell" {
		t.Fatalf("unexpected driver payload: %#v", driver)
	}
}

// TestHandleSendDriverRaceUsesQueryString verifies the driver race endpoint filters all datasets for the requested driver.
func TestHandleSendDriverRaceUsesQueryString(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/driver?driver_number=63", nil)
	rec := httptest.NewRecorder()
	handler.HandleSendDriverRace(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if int(payload["driver_number"].(float64)) != 63 {
		t.Fatalf("unexpected driver number: %#v", payload["driver_number"])
	}

	counts := payload["counts"].(map[string]any)
	if int(counts["laps"].(float64)) != 2 {
		t.Fatalf("unexpected laps count: %#v", counts["laps"])
	}
}

// TestHandleSendRaceStandings verifies standings snapshots are ordered and filtered by timestamp.
func TestHandleSendRaceStandings(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/standings/race?at=2025-03-16T04:15:00Z", nil)
	rec := httptest.NewRecorder()
	handler.HandleSendRaceStandings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	data := payload["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("unexpected standings count: %d", len(data))
	}

	first := data[0].(map[string]any)
	second := data[1].(map[string]any)
	if int(first["driver_number"].(float64)) != 1 || int(second["driver_number"].(float64)) != 63 {
		t.Fatalf("unexpected standings order: %#v", data)
	}
}

// TestHandleSendRaceStandingsInvalidTime verifies malformed snapshot timestamps are rejected.
func TestHandleSendRaceStandingsInvalidTime(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/standings/race?at=bad-date", nil)
	rec := httptest.NewRecorder()
	handler.HandleSendRaceStandings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorContains(t, rec, "invalid at query parameter")
}

// TestHandleListEventSessionsNotFound verifies missing event identifiers return HTTP 404.
func TestHandleListEventSessionsNotFound(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		ListEventSessionsFn: func(ctx context.Context, eventID string) ([]domain.SessionSummary, bool, error) {
			return nil, false, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/missing/sessions", nil)
	req.SetPathValue("eventId", "missing")
	rec := httptest.NewRecorder()
	handler.HandleListEventSessions(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

// TestHandleListChampionshipEvents verifies one championship code returns its stored events.
func TestHandleListChampionshipEvents(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetChampionshipEventsFn: func(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.EventSummary, bool, error) {
			return mocks.SampleChampionship(), []domain.EventSummary{mocks.SampleEvent()}, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/championships/f1/events", nil)
	req.SetPathValue("code", "f1")
	rec := httptest.NewRecorder()
	handler.HandleListChampionshipEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if int(payload["count"].(float64)) != 1 {
		t.Fatalf("unexpected events count: %#v", payload["count"])
	}
}

// TestHandleGetEventCatalog verifies one event summary can be fetched by route identifier.
func TestHandleGetEventCatalog(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetEventFn: func(ctx context.Context, eventID string) (domain.EventSummary, bool, error) {
			return mocks.SampleEvent(), true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/event-aus-2025", nil)
	req.SetPathValue("eventId", "event-aus-2025")
	rec := httptest.NewRecorder()
	handler.HandleGetEventCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandleGetSessionCatalog verifies one session summary can be fetched by route identifier.
func TestHandleGetSessionCatalog(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetSessionFn: func(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error) {
			return mocks.SampleSession(), true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-race-9693", nil)
	req.SetPathValue("sessionId", "session-race-9693")
	rec := httptest.NewRecorder()
	handler.HandleGetSessionCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandleSendSessionArchive verifies one stored session archive can be fetched explicitly by session identifier.
func TestHandleSendSessionArchive(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetSessionMergedFn: func(ctx context.Context, sessionID string) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-race-9693/archive", nil)
	req.SetPathValue("sessionId", "session-race-9693")
	rec := httptest.NewRecorder()
	handler.HandleSendSessionArchive(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload domain.RaceArchive
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode archive: %v", err)
	}
	if payload.Metadata.RaceSessKey != 9693 {
		t.Fatalf("unexpected archive payload: %#v", payload.Metadata)
	}
}

// TestHandleListSessionDrivers verifies session-scoped driver listing uses the requested session archive.
func TestHandleListSessionDrivers(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetSessionMergedFn: func(ctx context.Context, sessionID string) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-race-9693/drivers", nil)
	req.SetPathValue("sessionId", "session-race-9693")
	rec := httptest.NewRecorder()
	handler.HandleListSessionDrivers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if int(payload["count"].(float64)) != 2 {
		t.Fatalf("unexpected session drivers payload: %#v", payload)
	}
}

// TestHandleSendSessionBroadcast verifies session-scoped broadcast lookups return the stored session URL.
func TestHandleSendSessionBroadcast(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetSessionFn: func(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error) {
			return mocks.SampleSession(), true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-race-9693/broadcast", nil)
	req.SetPathValue("sessionId", "session-race-9693")
	rec := httptest.NewRecorder()
	handler.HandleSendSessionBroadcast(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if !strings.Contains(payload["broadcast_url"].(string), "session=9693") {
		t.Fatalf("unexpected session broadcast payload: %#v", payload)
	}
}

// TestHandleSendSessionDriverBroadcast verifies driver-specific session broadcast lookups return the stored URL.
func TestHandleSendSessionDriverBroadcast(t *testing.T) {
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetSessionDriverBroadcastFn: func(ctx context.Context, sessionID string, driverNumber int) (string, bool, error) {
			return "https://www.youtube.com/watch?v=dQw4w9WgXcQ&session=9693&driver=63", true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-race-9693/drivers/63/broadcast", nil)
	req.SetPathValue("sessionId", "session-race-9693")
	req.SetPathValue("driverNumber", "63")
	rec := httptest.NewRecorder()
	handler.HandleSendSessionDriverBroadcast(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if !strings.Contains(payload["broadcast_url"].(string), "driver=63") {
		t.Fatalf("unexpected driver broadcast payload: %#v", payload)
	}
}

// TestHandleSendDriverDatasetAlias verifies dataset aliases such as telemetry resolve correctly.
func TestHandleSendDriverDatasetAlias(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/race/drivers/63/telemetry", nil)
	req.SetPathValue("driverNumber", "63")
	req.SetPathValue("dataset", "telemetry")
	rec := httptest.NewRecorder()
	handler.HandleSendDriverDataset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if payload["dataset"] != "car_data" {
		t.Fatalf("unexpected dataset alias resolution: %#v", payload["dataset"])
	}
	if int(payload["count"].(float64)) != 1 {
		t.Fatalf("unexpected filtered row count: %#v", payload["count"])
	}
}

// TestHandleSendSessionDriverDatasetAlias verifies session-scoped driver dataset aliases resolve from the path.
func TestHandleSendSessionDriverDatasetAlias(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetSessionMergedFn: func(ctx context.Context, sessionID string) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-race-9693/drivers/63/laps", nil)
	req.SetPathValue("sessionId", "session-race-9693")
	req.SetPathValue("driverNumber", "63")
	rec := httptest.NewRecorder()
	handler.HandleSendSessionDriverDataset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if payload["dataset"] != "laps" {
		t.Fatalf("unexpected dataset alias resolution: %#v", payload["dataset"])
	}
	if int(payload["count"].(float64)) != 2 {
		t.Fatalf("unexpected filtered row count: %#v", payload["count"])
	}
}

// TestHandleSendSessionDriverLapLocation verifies lap-scoped location extraction returns only samples inside the lap window.
func TestHandleSendSessionDriverLapLocation(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetSessionMergedFn: func(ctx context.Context, sessionID string) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-race-9693/drivers/63/laps/1/location", nil)
	req.SetPathValue("sessionId", "session-race-9693")
	req.SetPathValue("driverNumber", "63")
	req.SetPathValue("lapNumber", "1")
	rec := httptest.NewRecorder()
	handler.HandleSendSessionDriverLapLocation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	if payload["dataset"] != "location" {
		t.Fatalf("unexpected dataset: %#v", payload["dataset"])
	}
	if int(payload["count"].(float64)) != 2 {
		t.Fatalf("unexpected lap location count: %#v", payload["count"])
	}

	data := payload["data"].([]any)
	first := data[0].(map[string]any)
	if first["driver_name"] == nil || first["team_name"] == nil {
		t.Fatalf("expected enriched location payload: %#v", first)
	}
}

// TestHandleSendChampionshipAndUtilityDatasets verifies global data endpoints expose the expected dataset names.
func TestHandleSendChampionshipAndUtilityDatasets(t *testing.T) {
	archive := mocks.SampleArchive()
	handler := newReadHandler(&mocks.RaceArchiveStoreMock{
		GetLatestMergedFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, archive.Metadata.GeneratedAt, true, nil
		},
	})

	cases := []struct {
		name           string
		call           func(http.ResponseWriter, *http.Request)
		target         string
		expectedStatus int
		expectedValue  string
		valueKey       string
	}{
		{
			name:           "driver championship",
			call:           handler.HandleSendDriverChampionship,
			target:         "/api/v1/race/championship/drivers",
			expectedStatus: http.StatusOK,
			expectedValue:  "championship_drivers",
			valueKey:       "dataset",
		},
		{
			name:           "constructor championship",
			call:           handler.HandleSendConstructorChampionship,
			target:         "/api/v1/race/championship/constructors",
			expectedStatus: http.StatusOK,
			expectedValue:  "championship_teams",
			valueKey:       "dataset",
		},
		{
			name:           "weather",
			call:           handler.HandleSendWeather,
			target:         "/api/v1/race/weather",
			expectedStatus: http.StatusOK,
			expectedValue:  "weather",
			valueKey:       "dataset",
		},
		{
			name:           "facts",
			call:           handler.HandleSendRaceFacts,
			target:         "/api/v1/race/facts",
			expectedStatus: http.StatusOK,
			expectedValue:  "race_control",
			valueKey:       "dataset",
		},
		{
			name:           "video url",
			call:           handler.HandleSendVideoURL,
			target:         "/api/v1/race/video-url",
			expectedStatus: http.StatusOK,
			expectedValue:  "youtube",
			valueKey:       "provider",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			rec := httptest.NewRecorder()
			tc.call(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			decodeJSON(t, rec, &payload)
			if payload[tc.valueKey] != tc.expectedValue {
				t.Fatalf("unexpected %s: %#v", tc.valueKey, payload[tc.valueKey])
			}
		})
	}
}

// newReadHandler builds a handler backed by a mocked read store.
func newReadHandler(store *mocks.RaceArchiveStoreMock) *api.RaceHandler {
	return api.NewRaceHandler(
		service.NewRaceService(nil, store),
		api.RaceDefaults{},
		2*time.Second,
	)
}

// decodeJSON decodes a JSON response recorder body into the given destination.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, dest any) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), dest); err != nil {
		t.Fatalf("failed to decode json: %v body=%s", err, rec.Body.String())
	}
}

// assertErrorContains verifies a JSON error response contains the expected substring.
func assertErrorContains(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()

	var payload map[string]any
	decodeJSON(t, rec, &payload)
	got := payload["error"].(string)
	if !strings.Contains(got, want) {
		t.Fatalf("expected error to contain %q, got %q", want, got)
	}
}
