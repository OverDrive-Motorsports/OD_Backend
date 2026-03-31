/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_service_test.go - Unit tests for race service orchestration and catalog delegation.
##
*/

package service_test

import (
	"context"
	"errors"
	"net/http"
	"path"
	"strings"
	"testing"
	"time"

	"overdrive/internal/domain"
	"overdrive/internal/providers/openf1"
	"overdrive/internal/service"
	"overdrive/internal/usecase"
	"overdrive/tests/internal/mocks"
)

// TestFetchAndStorePersistsArchive verifies the service builds an archive and persists it through the store.
func TestFetchAndStorePersistsArchive(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = mocks.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		endpoint := path.Base(r.URL.Path)

		switch endpoint {
		case "meetings":
			return mocks.JSONResponse(http.StatusOK, `[{"meeting_key":1254,"meeting_name":"Australian Grand Prix","country_name":"Australia"}]`), nil
		case "sessions":
			return mocks.JSONResponse(http.StatusOK, `[{"session_key":9693,"session_name":"Race","date_start":"2025-03-16T04:00:00Z"}]`), nil
		case "drivers":
			return mocks.JSONResponse(http.StatusOK, `[{"driver_number":81,"full_name":"Oscar Piastri","team_name":"McLaren"}]`), nil
		default:
			return mocks.JSONResponse(http.StatusOK, `[]`), nil
		}
	})
	defer func() { http.DefaultTransport = oldTransport }()

	builder := usecase.NewRaceBuilder(openf1.NewClient("http://openf1.test", time.Second, 0, 0, 0))
	var stored bool
	var storedArchive domain.RaceArchive
	store := &mocks.RaceArchiveStoreMock{
		StoreFn: func(ctx context.Context, archive domain.RaceArchive) (time.Time, error) {
			stored = true
			storedArchive = archive
			return time.Date(2025, 3, 16, 6, 5, 0, 0, time.UTC), nil
		},
	}

	svc := service.NewRaceService(builder, store)
	archive, err := svc.FetchAndStore(context.Background(), usecase.RaceBuildInput{
		Year:         2025,
		CountryName:  "Australia",
		MeetingName:  "Australian Grand Prix",
		DriverNumber: 81,
	})
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	if !stored {
		t.Fatal("expected archive to be stored")
	}
	if storedArchive.Metadata.DriverNumber != 81 {
		t.Fatalf("unexpected stored driver number: %d", storedArchive.Metadata.DriverNumber)
	}
	if archive.Metadata.MeetingKey != 1254 {
		t.Fatalf("unexpected meeting key: %d", archive.Metadata.MeetingKey)
	}
}

// TestGetLatestMergedStoredNotFound verifies the service returns not found without wrapping it as an error.
func TestGetLatestMergedStoredNotFound(t *testing.T) {
	svc := service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{})

	_, _, found, err := svc.GetLatestMergedStored(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected merged archive to be absent")
	}
}

// TestGetLatestStoredDelegates verifies the service reads the latest raw archive from storage.
func TestGetLatestStoredDelegates(t *testing.T) {
	archive := mocks.SampleArchive()
	storedAt := archive.Metadata.GeneratedAt
	svc := service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{
		GetLatestFn: func(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
			return archive, storedAt, true, nil
		},
	})

	got, gotStoredAt, found, err := svc.GetLatestStored(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected archive to be found")
	}
	if got.Metadata.MeetingKey != archive.Metadata.MeetingKey || !gotStoredAt.Equal(storedAt) {
		t.Fatalf("unexpected archive response: %#v %v", got, gotStoredAt)
	}
}

// TestFetchAndStoreWrapsStoreError verifies repository persistence failures are wrapped with context.
func TestFetchAndStoreWrapsStoreError(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = mocks.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		endpoint := path.Base(r.URL.Path)
		switch endpoint {
		case "meetings":
			return mocks.JSONResponse(http.StatusOK, `[{"meeting_key":1254,"meeting_name":"Australian Grand Prix","country_name":"Australia"}]`), nil
		case "sessions":
			return mocks.JSONResponse(http.StatusOK, `[{"session_key":9693,"session_name":"Race","date_start":"2025-03-16T04:00:00Z"}]`), nil
		case "drivers":
			return mocks.JSONResponse(http.StatusOK, `[{"driver_number":81,"full_name":"Oscar Piastri","team_name":"McLaren"}]`), nil
		default:
			return mocks.JSONResponse(http.StatusOK, `[]`), nil
		}
	})
	defer func() { http.DefaultTransport = oldTransport }()

	builder := usecase.NewRaceBuilder(openf1.NewClient("http://openf1.test", time.Second, 0, 0, 0))
	svc := service.NewRaceService(builder, &mocks.RaceArchiveStoreMock{
		StoreFn: func(ctx context.Context, archive domain.RaceArchive) (time.Time, error) {
			return time.Time{}, errors.New("insert failed")
		},
	})

	_, err := svc.FetchAndStore(context.Background(), usecase.RaceBuildInput{
		Year:         2025,
		CountryName:  "Australia",
		MeetingName:  "Australian Grand Prix",
		DriverNumber: 81,
	})
	if err == nil {
		t.Fatal("expected store error")
	}
	if !strings.Contains(err.Error(), "archive store failed") {
		t.Fatalf("expected wrapped store error, got %v", err)
	}
}

// TestCatalogDelegation verifies catalog reads are delegated to the repository mock.
func TestCatalogDelegation(t *testing.T) {
	svc := service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{
		ListChampionshipsFn: func(ctx context.Context) ([]domain.ChampionshipSummary, error) {
			return []domain.ChampionshipSummary{mocks.SampleChampionship()}, nil
		},
		GetChampionshipEventsFn: func(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.EventSummary, bool, error) {
			return mocks.SampleChampionship(), []domain.EventSummary{mocks.SampleEvent()}, true, nil
		},
		ListEventSessionsFn: func(ctx context.Context, eventID string) ([]domain.SessionSummary, bool, error) {
			return []domain.SessionSummary{mocks.SampleSession()}, true, nil
		},
	})

	championships, err := svc.ListChampionships(context.Background())
	if err != nil {
		t.Fatalf("unexpected championship list error: %v", err)
	}
	if len(championships) != 1 || championships[0].Code != "f1" {
		t.Fatalf("unexpected championships payload: %#v", championships)
	}

	championship, events, found, err := svc.GetChampionshipEvents(context.Background(), "f1")
	if err != nil {
		t.Fatalf("unexpected championship events error: %v", err)
	}
	if !found || championship.Code != "f1" || len(events) != 1 {
		t.Fatalf("unexpected championship events payload: %#v %#v %v", championship, events, found)
	}

	sessions, found, err := svc.ListEventSessions(context.Background(), "event-aus-2025")
	if err != nil {
		t.Fatalf("unexpected session list error: %v", err)
	}
	if !found || len(sessions) != 1 || sessions[0].ID != "session-race-9693" {
		t.Fatalf("unexpected session payload: %#v %v", sessions, found)
	}
}

// TestGetEventAndSessionDelegation verifies single-record catalog lookups are delegated to the repository.
func TestGetEventAndSessionDelegation(t *testing.T) {
	svc := service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{
		GetEventFn: func(ctx context.Context, eventID string) (domain.EventSummary, bool, error) {
			return mocks.SampleEvent(), true, nil
		},
		GetSessionFn: func(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error) {
			return mocks.SampleSession(), true, nil
		},
	})

	event, found, err := svc.GetEvent(context.Background(), "event-aus-2025")
	if err != nil {
		t.Fatalf("unexpected event lookup error: %v", err)
	}
	if !found || event.ID != "event-aus-2025" {
		t.Fatalf("unexpected event payload: %#v %v", event, found)
	}

	session, found, err := svc.GetSession(context.Background(), "session-race-9693")
	if err != nil {
		t.Fatalf("unexpected session lookup error: %v", err)
	}
	if !found || session.ID != "session-race-9693" {
		t.Fatalf("unexpected session payload: %#v %v", session, found)
	}
}

// TestSessionArchiveAndBroadcastDelegation verifies session-scoped archive and broadcast reads are delegated.
func TestSessionArchiveAndBroadcastDelegation(t *testing.T) {
	archive := mocks.SampleArchive()
	storedAt := archive.Metadata.GeneratedAt
	svc := service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{
		GetSessionMergedFn: func(ctx context.Context, sessionID string) (domain.RaceArchive, time.Time, bool, error) {
			return archive, storedAt, true, nil
		},
		GetSessionMetadataFn: func(ctx context.Context, sessionID string) (domain.SessionMetadataWindow, bool, error) {
			return domain.SessionMetadataWindow{
				Metadata: map[string]any{"meeting_name": "Australian Grand Prix"},
				StoredAt: storedAt,
			}, true, nil
		},
		GetSessionDatasetCatalogFn: func(ctx context.Context, sessionID string) (domain.SessionDatasetCatalogWindow, bool, error) {
			return domain.SessionDatasetCatalogWindow{
				SessionID: sessionID,
				Metadata:  map[string]any{"meeting_name": "Australian Grand Prix"},
				Count:     1,
				Data:      []map[string]any{{"dataset": "drivers", "count": 2}},
			}, true, nil
		},
		GetSessionDriverBroadcastFn: func(ctx context.Context, sessionID string, driverNumber int) (string, bool, error) {
			return "https://www.youtube.com/watch?v=dQw4w9WgXcQ&session=9693&driver=63", true, nil
		},
		GetSessionDatasetFn: func(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetWindow, bool, error) {
			return domain.SessionDatasetWindow{
				Dataset:   dataset,
				SessionID: sessionID,
				Count:     2,
				Data:      []map[string]any{{"driver_number": 63}, {"driver_number": 1}},
			}, true, nil
		},
		GetSessionDriverDatasetFn: func(ctx context.Context, sessionID string, driverNumber int, dataset string) (domain.DriverDatasetWindow, bool, error) {
			return domain.DriverDatasetWindow{
				Dataset:      dataset,
				SessionID:    sessionID,
				DriverNumber: driverNumber,
				DriverName:   "George Russell",
				TeamName:     "Mercedes",
				Count:        1,
				Data:         []map[string]any{{"driver_number": 63, "lap_number": 27}},
			}, true, nil
		},
		GetSessionRaceStandingsFn: func(ctx context.Context, sessionID string, at *time.Time) (domain.RaceStandingsWindow, bool, error) {
			return domain.RaceStandingsWindow{
				Dataset:    "position",
				SessionID:  sessionID,
				SnapshotAt: time.Date(2025, 3, 16, 4, 0, 0, 0, time.UTC),
				Count:      1,
				Data:       []map[string]any{{"driver_number": 63, "position": 1}},
			}, true, nil
		},
	})

	gotArchive, gotStoredAt, found, err := svc.GetSessionMergedStored(context.Background(), "session-race-9693")
	if err != nil {
		t.Fatalf("unexpected session archive error: %v", err)
	}
	if !found || gotArchive.Metadata.RaceSessKey != 9693 || !gotStoredAt.Equal(storedAt) {
		t.Fatalf("unexpected session archive payload: %#v %v %v", gotArchive, gotStoredAt, found)
	}

	metadataWindow, direct, err := svc.GetSessionMetadata(context.Background(), "session-race-9693")
	if err != nil {
		t.Fatalf("unexpected session metadata error: %v", err)
	}
	if !direct || metadataWindow.Metadata["meeting_name"] != "Australian Grand Prix" {
		t.Fatalf("unexpected session metadata payload: %#v direct=%v", metadataWindow, direct)
	}

	catalog, direct, err := svc.GetSessionDatasetCatalog(context.Background(), "session-race-9693")
	if err != nil {
		t.Fatalf("unexpected session dataset catalog error: %v", err)
	}
	if !direct || catalog.Count != 1 {
		t.Fatalf("unexpected session dataset catalog payload: %#v direct=%v", catalog, direct)
	}

	url, found, err := svc.GetSessionDriverBroadcast(context.Background(), "session-race-9693", 63)
	if err != nil {
		t.Fatalf("unexpected session broadcast error: %v", err)
	}
	if !found || !strings.Contains(url, "driver=63") {
		t.Fatalf("unexpected broadcast payload: %q found=%v", url, found)
	}

	sessionDataset, direct, err := svc.GetSessionDataset(context.Background(), "session-race-9693", "drivers")
	if err != nil {
		t.Fatalf("unexpected session dataset error: %v", err)
	}
	if !direct || sessionDataset.Dataset != "drivers" || sessionDataset.Count != 2 {
		t.Fatalf("unexpected direct session dataset payload: %#v direct=%v", sessionDataset, direct)
	}

	window, direct, err := svc.GetSessionDriverDataset(context.Background(), "session-race-9693", 63, "laps")
	if err != nil {
		t.Fatalf("unexpected session driver dataset error: %v", err)
	}
	if !direct || window.Dataset != "laps" || window.DriverName != "George Russell" {
		t.Fatalf("unexpected direct dataset payload: %#v direct=%v", window, direct)
	}

	standings, direct, err := svc.GetSessionRaceStandings(context.Background(), "session-race-9693", nil)
	if err != nil {
		t.Fatalf("unexpected session standings error: %v", err)
	}
	if !direct || standings.Dataset != "position" || standings.Count != 1 {
		t.Fatalf("unexpected direct standings payload: %#v direct=%v", standings, direct)
	}
}
