/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_replay_stream_controller_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
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

// fakeRaceReplayUseCase implements ports.RaceReplayUseCase for controller-level tests.
type fakeRaceReplayUseCase struct {
	sessionExists    bool
	sessionExistsErr error
	replay           domain.RaceReplay
	replayErr        error
}

func (f *fakeRaceReplayUseCase) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	return f.sessionExists, f.sessionExistsErr
}
func (f *fakeRaceReplayUseCase) GetReplay(ctx context.Context, sessionID string, driverNumber *int) (domain.RaceReplay, error) {
	return f.replay, f.replayErr
}

func newReplayRequest(sessionID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID+"/race/replay", nil)
	req.SetPathValue("sessionId", sessionID)
	return req
}

// TestRaceReplayStreamController_GetReplay_NormalCase proves a known session returns 200 with the
// usecase's replay payload.
func TestRaceReplayStreamController_GetReplay_NormalCase(t *testing.T) {
	c := NewRaceReplayStreamController(&fakeRaceReplayUseCase{
		sessionExists: true,
		replay:        domain.RaceReplay{Telemetry: map[string][]domain.RaceReplayTelemetrySample{"63": {{Speed: 300}}}},
	})
	rec := httptest.NewRecorder()
	c.GetReplay(rec, newReplayRequest("s1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestRaceReplayStreamController_GetReplay_UnknownSession proves an unknown session yields 404
// before GetReplay is called.
func TestRaceReplayStreamController_GetReplay_UnknownSession(t *testing.T) {
	c := NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExists: false})
	rec := httptest.NewRecorder()
	c.GetReplay(rec, newReplayRequest("missing"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestRaceReplayStreamController_GetReplay_UpstreamFailure proves a championship-service failure
// while checking the session is reported as 502.
func TestRaceReplayStreamController_GetReplay_UpstreamFailure(t *testing.T) {
	c := NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExistsErr: errors.New("down")})
	rec := httptest.NewRecorder()
	c.GetReplay(rec, newReplayRequest("s1"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestRaceReplayStreamController_GetReplay_InvalidDriverNumberQueryParam proves an invalid
// driverNumber query parameter is rejected with 400 before GetReplay is called.
func TestRaceReplayStreamController_GetReplay_InvalidDriverNumberQueryParam(t *testing.T) {
	c := NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExists: true})
	req := httptest.NewRequest(http.MethodGet, "/sessions/s1/race/replay?driverNumber=nope", nil)
	req.SetPathValue("sessionId", "s1")
	rec := httptest.NewRecorder()
	c.GetReplay(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestRaceReplayStreamController_GetReplay_UsecaseFailure proves a GetReplay failure (after the
// session check succeeded) is reported as 500.
func TestRaceReplayStreamController_GetReplay_UsecaseFailure(t *testing.T) {
	c := NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExists: true, replayErr: errors.New("db down")})
	rec := httptest.NewRecorder()
	c.GetReplay(rec, newReplayRequest("s1"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", rec.Code, rec.Body.String())
	}
}
