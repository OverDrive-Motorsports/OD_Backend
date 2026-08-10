/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## telemetry_controller_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"overdrive/services/race-data-service/src/core/domain"
)

// fakeTelemetryUseCase implements ports.TelemetryUseCase for controller-level tests.
type fakeTelemetryUseCase struct {
	sessionExists    bool
	sessionExistsErr error

	speed        []domain.TelemetrySpeed
	speedErr     error
	engine       []domain.TelemetryEngine
	engineErr    error
	location     []domain.TelemetryLocation
	locationErr  error
	intervals    *domain.TelemetryIntervals
	intervalsErr error
}

func (f *fakeTelemetryUseCase) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	return f.sessionExists, f.sessionExistsErr
}
func (f *fakeTelemetryUseCase) GetSpeed(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetrySpeed, error) {
	return f.speed, f.speedErr
}
func (f *fakeTelemetryUseCase) GetEngine(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryEngine, error) {
	return f.engine, f.engineErr
}
func (f *fakeTelemetryUseCase) GetLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryLocation, error) {
	return f.location, f.locationErr
}
func (f *fakeTelemetryUseCase) GetIntervals(ctx context.Context, sessionID string, driverNumber int) (*domain.TelemetryIntervals, error) {
	return f.intervals, f.intervalsErr
}

func newTelemetryRequest(path, sessionID, driverNumber string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.SetPathValue("sessionId", sessionID)
	req.SetPathValue("driverNumber", driverNumber)
	return req
}

// TestTelemetryController_GetSpeed_NormalCase proves a valid request returns the usecase's samples
// unmodified with a 200.
func TestTelemetryController_GetSpeed_NormalCase(t *testing.T) {
	c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: true, speed: []domain.TelemetrySpeed{{Speed: 300}}})
	req := newTelemetryRequest("/sessions/s1/drivers/44/telemetry/speed", "s1", "44")
	rec := httptest.NewRecorder()
	c.GetSpeed(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestTelemetryController_ParseCommon_InvalidDriverNumber proves an invalid path driverNumber
// short-circuits with 400 before the session is even checked.
func TestTelemetryController_ParseCommon_InvalidDriverNumber(t *testing.T) {
	c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: true})
	req := newTelemetryRequest("/sessions/s1/drivers/abc/telemetry/speed", "s1", "abc")
	rec := httptest.NewRecorder()
	c.GetSpeed(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestTelemetryController_ParseCommon_UnknownSession proves requireSession's 404 short-circuits
// before the repository is consulted.
func TestTelemetryController_ParseCommon_UnknownSession(t *testing.T) {
	c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: false})
	req := newTelemetryRequest("/sessions/missing/drivers/44/telemetry/speed", "missing", "44")
	rec := httptest.NewRecorder()
	c.GetSpeed(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestTelemetryController_ParseCommon_UpstreamFailure proves a championship-service failure while
// verifying the session is reported as 502 (UpstreamUnavailable), distinct from a local repository
// failure.
func TestTelemetryController_ParseCommon_UpstreamFailure(t *testing.T) {
	c := NewTelemetryController(&fakeTelemetryUseCase{sessionExistsErr: errors.New("down")})
	req := newTelemetryRequest("/sessions/s1/drivers/44/telemetry/speed", "s1", "44")
	rec := httptest.NewRecorder()
	c.GetSpeed(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestTelemetryController_ParseCommon_InvalidLapQueryParam proves an invalid lapNumber query
// parameter is rejected with 400.
func TestTelemetryController_ParseCommon_InvalidLapQueryParam(t *testing.T) {
	c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: true})
	req := httptest.NewRequest(http.MethodGet, "/sessions/s1/drivers/44/telemetry/speed?lapNumber=nope", nil)
	req.SetPathValue("sessionId", "s1")
	req.SetPathValue("driverNumber", "44")
	rec := httptest.NewRecorder()
	c.GetSpeed(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestTelemetryController_GetEngine_RepositoryFailure proves a repository failure surfaces as 500.
func TestTelemetryController_GetEngine_RepositoryFailure(t *testing.T) {
	c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: true, engineErr: errors.New("db down")})
	req := newTelemetryRequest("/sessions/s1/drivers/44/telemetry/engine", "s1", "44")
	rec := httptest.NewRecorder()
	c.GetEngine(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestTelemetryController_GetLocation_NormalCase proves the location endpoint follows the same
// parseCommon/pass-through path as speed/engine.
func TestTelemetryController_GetLocation_NormalCase(t *testing.T) {
	c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: true, location: []domain.TelemetryLocation{{X: 1}}})
	req := newTelemetryRequest("/sessions/s1/drivers/44/telemetry/location", "s1", "44")
	rec := httptest.NewRecorder()
	c.GetLocation(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestTelemetryController_GetIntervals_ShapeAndNotFound proves GetIntervals (which uses
// requireSession directly, not parseCommon, since it has no lapNumber param) returns the usecase's
// payload on success and a 404 when the usecase yields nil.
func TestTelemetryController_GetIntervals_ShapeAndNotFound(t *testing.T) {
	t.Run("present payload -> 200", func(t *testing.T) {
		c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: true, intervals: &domain.TelemetryIntervals{DriverNumber: 44}})
		req := newTelemetryRequest("/sessions/s1/drivers/44/telemetry/intervals", "s1", "44")
		rec := httptest.NewRecorder()
		c.GetIntervals(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("nil payload -> 404", func(t *testing.T) {
		c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: true, intervals: nil})
		req := newTelemetryRequest("/sessions/s1/drivers/44/telemetry/intervals", "s1", "44")
		rec := httptest.NewRecorder()
		c.GetIntervals(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid driverNumber path param -> 400", func(t *testing.T) {
		c := NewTelemetryController(&fakeTelemetryUseCase{sessionExists: true})
		req := newTelemetryRequest("/sessions/s1/drivers/abc/telemetry/intervals", "s1", "abc")
		rec := httptest.NewRecorder()
		c.GetIntervals(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}
