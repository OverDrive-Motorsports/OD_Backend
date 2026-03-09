/**
##
## OverDrive 2026
## All Technical rights reserved
##
## build_race_test.go - Unit tests for race archive building with mocked OpenF1 responses.
##
*/

package usecase_test

import (
	"context"
	"net/http"
	"path"
	"testing"
	"time"

	"overdrive/internal/providers/openf1"
	"overdrive/internal/usecase"
	"overdrive/tests/internal/mocks"
)

// TestBuildDriverFocusedSuccess verifies a single-driver archive is assembled from provider data.
func TestBuildDriverFocusedSuccess(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = mocks.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return openF1MockResponse(t, r, false), nil
	})
	defer func() { http.DefaultTransport = oldTransport }()

	builder := usecase.NewRaceBuilder(openf1.NewClient("http://openf1.test", time.Second, 0, 0, 0))
	archive, err := builder.Build(context.Background(), usecase.RaceBuildInput{
		Year:         2025,
		CountryName:  "Australia",
		MeetingName:  "Australian Grand Prix",
		DriverNumber: 81,
	})
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	if archive.Metadata.DriverNumber != 81 {
		t.Fatalf("unexpected driver number: %d", archive.Metadata.DriverNumber)
	}
	if len(archive.Datasets["drivers"]) != 1 {
		t.Fatalf("unexpected driver dataset length: %d", len(archive.Datasets["drivers"]))
	}
	if archive.Counts["drivers"] != 1 {
		t.Fatalf("unexpected driver count: %d", archive.Counts["drivers"])
	}
	if archive.FetchErrors != nil {
		t.Fatalf("expected nil fetch errors, got: %#v", archive.FetchErrors)
	}
}

// TestBuildFullRaceFallsBackByDriver verifies heavy endpoint 422 responses are recovered per driver.
func TestBuildFullRaceFallsBackByDriver(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = mocks.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return openF1MockResponse(t, r, true), nil
	})
	defer func() { http.DefaultTransport = oldTransport }()

	builder := usecase.NewRaceBuilder(openf1.NewClient("http://openf1.test", time.Second, 0, 0, 0))
	archive, err := builder.Build(context.Background(), usecase.RaceBuildInput{
		Year:        2025,
		CountryName: "Australia",
		MeetingName: "Australian Grand Prix",
	})
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	if archive.Counts["car_data"] != 2 {
		t.Fatalf("unexpected fallback car_data count: %d", archive.Counts["car_data"])
	}
	if archive.Counts["team_radio"] != 0 {
		t.Fatalf("unexpected team_radio count: %d", archive.Counts["team_radio"])
	}
	if archive.FetchErrors != nil {
		t.Fatalf("expected nil fetch errors after fallback, got: %#v", archive.FetchErrors)
	}
}

// openF1MockResponse serves deterministic dataset fixtures for build tests.
func openF1MockResponse(t *testing.T, r *http.Request, fallback bool) *http.Response {
	t.Helper()

	endpoint := path.Base(r.URL.Path)
	driver := r.URL.Query().Get("driver_number")

	switch endpoint {
	case "meetings":
		return mocks.JSONResponse(http.StatusOK, `[{"meeting_key":1254,"meeting_name":"Australian Grand Prix","meeting_official_name":"Formula 1 Australian Grand Prix","country_name":"Australia"}]`)
	case "sessions":
		return mocks.JSONResponse(http.StatusOK, `[
			{"session_key":9691,"session_name":"Practice 1","date_start":"2025-03-14T01:00:00Z"},
			{"session_key":9693,"session_name":"Race","date_start":"2025-03-16T04:00:00Z","date_end":"2025-03-16T06:00:00Z"}
		]`)
	case "drivers":
		return mocks.JSONResponse(http.StatusOK, `[
			{"driver_number":81,"full_name":"Oscar Piastri","team_name":"McLaren","name_acronym":"PIA"},
			{"driver_number":1,"full_name":"Max Verstappen","team_name":"Red Bull Racing","name_acronym":"VER"}
		]`)
	case "car_data":
		if fallback && driver == "" {
			return mocks.JSONResponse(http.StatusUnprocessableEntity, `{"detail":"Failed to retrieve information. You're likely asking for too much data at once."}`)
		}
		switch driver {
		case "81":
			return mocks.JSONResponse(http.StatusOK, `[{"driver_number":81,"date":"2025-03-16T04:00:00Z","speed":299}]`)
		case "1":
			return mocks.JSONResponse(http.StatusOK, `[{"driver_number":1,"date":"2025-03-16T04:00:00Z","speed":301}]`)
		default:
			return mocks.JSONResponse(http.StatusOK, `[]`)
		}
	case "team_radio":
		return mocks.JSONResponse(http.StatusNotFound, `{"detail":"No results found."}`)
	default:
		switch driver {
		case "81":
			return mocks.JSONResponse(http.StatusOK, `[{"driver_number":81}]`)
		case "1":
			return mocks.JSONResponse(http.StatusOK, `[{"driver_number":1}]`)
		default:
			return mocks.JSONResponse(http.StatusOK, `[]`)
		}
	}
}
