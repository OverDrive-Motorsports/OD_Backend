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
		GetChampionshipRacesFn: func(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.RaceSummary, bool, error) {
			return mocks.SampleChampionship(), []domain.RaceSummary{mocks.SampleRace()}, true, nil
		},
		ListRaceSessionsFn: func(ctx context.Context, raceID string) ([]domain.SessionSummary, bool, error) {
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

	championship, races, found, err := svc.GetChampionshipRaces(context.Background(), "f1")
	if err != nil {
		t.Fatalf("unexpected championship races error: %v", err)
	}
	if !found || championship.Code != "f1" || len(races) != 1 {
		t.Fatalf("unexpected championship races payload: %#v %#v %v", championship, races, found)
	}

	sessions, found, err := svc.ListRaceSessions(context.Background(), "race-aus-2025")
	if err != nil {
		t.Fatalf("unexpected session list error: %v", err)
	}
	if !found || len(sessions) != 1 || sessions[0].ID != "session-race-9693" {
		t.Fatalf("unexpected session payload: %#v %v", sessions, found)
	}
}

// TestGetRaceAndSessionDelegation verifies single-record catalog lookups are delegated to the repository.
func TestGetRaceAndSessionDelegation(t *testing.T) {
	svc := service.NewRaceService(nil, &mocks.RaceArchiveStoreMock{
		GetRaceFn: func(ctx context.Context, raceID string) (domain.RaceSummary, bool, error) {
			return mocks.SampleRace(), true, nil
		},
		GetSessionFn: func(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error) {
			return mocks.SampleSession(), true, nil
		},
	})

	race, found, err := svc.GetRace(context.Background(), "race-aus-2025")
	if err != nil {
		t.Fatalf("unexpected race lookup error: %v", err)
	}
	if !found || race.ID != "race-aus-2025" {
		t.Fatalf("unexpected race payload: %#v %v", race, found)
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
		GetSessionDriverBroadcastFn: func(ctx context.Context, sessionID string, driverNumber int) (string, bool, error) {
			return "https://www.youtube.com/watch?v=dQw4w9WgXcQ&session=9693&driver=63", true, nil
		},
	})

	gotArchive, gotStoredAt, found, err := svc.GetSessionMergedStored(context.Background(), "session-race-9693")
	if err != nil {
		t.Fatalf("unexpected session archive error: %v", err)
	}
	if !found || gotArchive.Metadata.RaceSessKey != 9693 || !gotStoredAt.Equal(storedAt) {
		t.Fatalf("unexpected session archive payload: %#v %v %v", gotArchive, gotStoredAt, found)
	}

	url, found, err := svc.GetSessionDriverBroadcast(context.Background(), "session-race-9693", 63)
	if err != nil {
		t.Fatalf("unexpected session broadcast error: %v", err)
	}
	if !found || !strings.Contains(url, "driver=63") {
		t.Fatalf("unexpected broadcast payload: %q found=%v", url, found)
	}
}
