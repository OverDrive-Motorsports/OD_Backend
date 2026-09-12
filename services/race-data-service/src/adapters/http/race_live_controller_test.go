/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_live_controller_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
##
*/

package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// fakeRaceLiveUseCase implements ports.RaceLiveUseCase for controller-level tests.
type fakeRaceLiveUseCase struct {
	sessionExists    bool
	sessionExistsErr error

	position    any
	positionErr error
	laps        any
	lapsErr     error
	stints      []domain.RaceStint
	stintsErr   error
	pitStops    []domain.RacePitStop
	pitStopsErr error
	weather     []domain.RaceWeatherSample
	weatherErr  error
	radio       []domain.RaceRadioMessage
	radioErr    error
	waitEvents  []domain.RaceControlEvent
	waitErr     error
}

func (f *fakeRaceLiveUseCase) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	return f.sessionExists, f.sessionExistsErr
}
func (f *fakeRaceLiveUseCase) GetPosition(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) (any, error) {
	return f.position, f.positionErr
}
func (f *fakeRaceLiveUseCase) GetLaps(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) (any, error) {
	return f.laps, f.lapsErr
}
func (f *fakeRaceLiveUseCase) GetStints(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceStint, error) {
	return f.stints, f.stintsErr
}
func (f *fakeRaceLiveUseCase) GetPitStops(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePitStop, error) {
	return f.pitStops, f.pitStopsErr
}
func (f *fakeRaceLiveUseCase) GetWeather(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error) {
	return f.weather, f.weatherErr
}
func (f *fakeRaceLiveUseCase) GetRadio(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error) {
	return f.radio, f.radioErr
}
func (f *fakeRaceLiveUseCase) WaitForRaceControl(ctx context.Context, sessionID string, timeout time.Duration) ([]domain.RaceControlEvent, error) {
	return f.waitEvents, f.waitErr
}

func newRaceLiveRequest(t *testing.T, sessionID string) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID+"/race/position", nil)
	req.SetPathValue("sessionId", sessionID)
	return req, httptest.NewRecorder()
}

// TestRaceLiveController_RequireSession_UnknownAndUpstreamFailure proves every handler goes
// through requireSession first: an unknown session yields 404 and an upstream failure checking the
// session yields 502 (UpstreamUnavailable), both before the usecase's own data method is ever
// consulted.
func TestRaceLiveController_RequireSession_UnknownAndUpstreamFailure(t *testing.T) {
	t.Run("unknown session -> 404", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: false})
		req, rec := newRaceLiveRequest(t, "missing")
		c.GetPosition(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("upstream failure -> 502", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExistsErr: errors.New("championship-service down")})
		req, rec := newRaceLiveRequest(t, "s1")
		c.GetPosition(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestRaceLiveController_GetPosition_ShapeBranch proves the controller writes whatever shape the
// usecase returns unmodified: a single object for a scoped driverNumber, and a 404 when the
// usecase reports nil (no data for that driver).
func TestRaceLiveController_GetPosition_ShapeBranch(t *testing.T) {
	t.Run("single object", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true, position: domain.RacePosition{DriverNumber: 44, Position: 2}})
		req, rec := newRaceLiveRequest(t, "s1")
		c.GetPosition(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var got domain.RacePosition
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.DriverNumber != 44 {
			t.Fatalf("expected a single position object, got %s (err %v)", rec.Body.String(), err)
		}
	})

	t.Run("nil payload -> 404", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true, position: nil})
		req, rec := newRaceLiveRequest(t, "s1")
		c.GetPosition(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("bare array", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true, position: []domain.RacePosition{{DriverNumber: 1}, {DriverNumber: 44}}})
		req, rec := newRaceLiveRequest(t, "s1")
		c.GetPosition(rec, req)
		var got []domain.RacePosition
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 2 {
			t.Fatalf("expected a 2-element array, got %s (err %v)", rec.Body.String(), err)
		}
	})
}

// TestRaceLiveController_GetLaps_ShapeBranch mirrors GetPosition's shape-branch coverage for
// /race/laps.
func TestRaceLiveController_GetLaps_ShapeBranch(t *testing.T) {
	t.Run("single object", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true, laps: domain.RaceLapsResponse{DriverNumber: 1}})
		req, rec := newRaceLiveRequest(t, "s1")
		c.GetLaps(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("nil payload -> 404", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true, laps: nil})
		req, rec := newRaceLiveRequest(t, "s1")
		c.GetLaps(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestRaceLiveController_ArrayEndpoints_PassThroughAndErrorMapping proves GetStints/GetPitStops/
// GetWeather/GetRadio write the usecase's array unmodified on success and map any error to a 500
// (apierror.Internal, matching classifyDatasetErr's non-dataset siblings).
func TestRaceLiveController_ArrayEndpoints_PassThroughAndErrorMapping(t *testing.T) {
	c := NewRaceLiveController(&fakeRaceLiveUseCase{
		sessionExists: true,
		stints:        []domain.RaceStint{{DriverNumber: 1}},
		pitStops:      []domain.RacePitStop{{DriverNumber: 1}},
		weather:       []domain.RaceWeatherSample{{}},
		radio:         []domain.RaceRadioMessage{{DriverNumber: 1}},
	})

	req, rec := newRaceLiveRequest(t, "s1")
	c.GetStints(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetStints status = %d, want 200", rec.Code)
	}

	req, rec = newRaceLiveRequest(t, "s1")
	c.GetPitStops(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetPitStops status = %d, want 200", rec.Code)
	}

	req, rec = newRaceLiveRequest(t, "s1")
	c.GetWeather(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetWeather status = %d, want 200", rec.Code)
	}

	req, rec = newRaceLiveRequest(t, "s1")
	c.GetRadio(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetRadio status = %d, want 200", rec.Code)
	}

	errC := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true, weatherErr: errors.New("db down")})
	req, rec = newRaceLiveRequest(t, "s1")
	errC.GetWeather(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GetWeather with a repository error: status = %d, want 500", rec.Code)
	}
}

// TestRaceLiveController_PostRaceControl proves the long-poll handler substitutes an empty JSON
// array (never null) when WaitForRaceControl returns a nil slice, and maps a failure to 500.
func TestRaceLiveController_PostRaceControl(t *testing.T) {
	t.Run("nil events become an empty array, not null", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true, waitEvents: nil})
		req, rec := newRaceLiveRequest(t, "s1")
		c.PostRaceControl(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "[]\n" {
			t.Fatalf("expected a bare empty array body, got %q", rec.Body.String())
		}
	})

	t.Run("broadcaster failure -> 500", func(t *testing.T) {
		c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true, waitErr: errors.New("boom")})
		req, rec := newRaceLiveRequest(t, "s1")
		c.PostRaceControl(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestRaceLiveController_GetPosition_InvalidQueryParam proves an invalid driverNumber query
// parameter is rejected with 400 before the usecase is ever called.
func TestRaceLiveController_GetPosition_InvalidQueryParam(t *testing.T) {
	c := NewRaceLiveController(&fakeRaceLiveUseCase{sessionExists: true})
	req := httptest.NewRequest(http.MethodGet, "/sessions/s1/race/position?driverNumber=not-a-number", nil)
	req.SetPathValue("sessionId", "s1")
	rec := httptest.NewRecorder()
	c.GetPosition(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}
