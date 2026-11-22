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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// fakeRaceReplayUseCase implements ports.RaceReplayUseCase: it emits parts, then returns err.
type fakeRaceReplayUseCase struct {
	sessionExists    bool
	sessionExistsErr error
	parts            []domain.ReplayPart
	err              error
	gotQuery         domain.ReplayQuery
	called           bool
}

func (f *fakeRaceReplayUseCase) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	return f.sessionExists, f.sessionExistsErr
}

func (f *fakeRaceReplayUseCase) StreamReplay(ctx context.Context, sessionID string, query domain.ReplayQuery, emit func(domain.ReplayPart) error) error {
	f.called = true
	f.gotQuery = query
	for _, part := range f.parts {
		if err := emit(part); err != nil {
			return err
		}
	}
	return f.err
}

func newReplayRequest(sessionID string) *http.Request {
	return newReplayRequestWithQuery(sessionID, "")
}

func newReplayRequestWithQuery(sessionID string, query string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID+"/race/replay"+query, nil)
	req.SetPathValue("sessionId", sessionID)
	return req
}

// driverPart builds a driver-scoped part from samples.
func driverPart(t *testing.T, dataset string, driver int, samples any) domain.ReplayPart {
	t.Helper()
	rows, err := json.Marshal(samples)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return domain.ReplayPart{Dataset: dataset, DriverNumber: &driver, Rows: rows}
}

// encodeLegacy returns what the former handler wrote for replay (json.Encoder output).
func encodeLegacy(t *testing.T, replay domain.RaceReplay) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(replay); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.String()
}

// TestRaceReplayStreamController_GetReplay_SameBytesAsBefore proves the streamed object is
// byte-for-byte what encoding the whole RaceReplay struct produced, for a dump with several
// drivers, missing datasets and track-wide arrays: the public contract is unchanged.
func TestRaceReplayStreamController_GetReplay_SameBytesAsBefore(t *testing.T) {
	telemetry11 := []domain.RaceReplayTelemetrySample{{Speed: 280, Timestamp: time.Date(2026, 3, 8, 15, 0, 1, 0, time.UTC)}}
	telemetry63 := []domain.RaceReplayTelemetrySample{{Speed: 300}, {Speed: 310, DrsActive: true}}
	radio63 := []domain.RaceReplayRadioSample{{AudioURL: "https://example.test/a.mp3?x=<y>&z"}}
	raceControl := []domain.RaceControlEvent{{Message: "green flag"}}
	uc := &fakeRaceReplayUseCase{sessionExists: true, parts: []domain.ReplayPart{
		driverPart(t, "telemetry", 11, telemetry11),
		driverPart(t, "telemetry", 63, telemetry63),
		driverPart(t, "radio", 63, radio63),
		{Dataset: "raceControl", Rows: mustJSON(t, raceControl)},
		{Dataset: "weather", Rows: json.RawMessage("[]")},
	}}
	rec := httptest.NewRecorder()

	NewRaceReplayStreamController(uc).GetReplay(rec, newReplayRequest("s1"))

	want := encodeLegacy(t, domain.RaceReplay{
		Telemetry:   map[string][]domain.RaceReplayTelemetrySample{"11": telemetry11, "63": telemetry63},
		Position:    map[string][]domain.RaceReplayPositionSample{},
		Radio:       map[string][]domain.RaceReplayRadioSample{"63": radio63},
		PitStop:     map[string][]domain.RaceReplayPitStopSample{},
		Stint:       map[string][]domain.RaceReplayStintSample{},
		Lap:         map[string][]domain.RaceReplayLapSample{},
		RaceControl: raceControl,
		Weather:     []domain.RaceWeatherSample{},
	})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status = %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != want {
		t.Fatalf("body =\n%s\nwant\n%s", rec.Body.String(), want)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// TestRaceReplayStreamController_GetReplay_NoPart proves a replay with no part at all still
// writes every key (empty objects and arrays), as a 200.
func TestRaceReplayStreamController_GetReplay_NoPart(t *testing.T) {
	rec := httptest.NewRecorder()
	NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExists: true}).GetReplay(rec, newReplayRequest("s1"))

	want := `{"telemetry":{},"position":{},"radio":{},"pitStop":{},"stint":{},"lap":{},"raceControl":[],"weather":[]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("got %d %q, want 200 %q", rec.Code, rec.Body.String(), want)
	}
}

// TestRaceReplayStreamController_GetReplay_QueryParsed proves driverNumber, fromUtc and toUtc
// reach the usecase.
func TestRaceReplayStreamController_GetReplay_QueryParsed(t *testing.T) {
	uc := &fakeRaceReplayUseCase{sessionExists: true}
	rec := httptest.NewRecorder()

	NewRaceReplayStreamController(uc).GetReplay(rec, newReplayRequestWithQuery("s1", "?driverNumber=44&fromUtc=2026-03-08T15:00:00Z&toUtc=2026-03-08T15:10:00.5Z"))

	q := uc.gotQuery
	if rec.Code != http.StatusOK || q.DriverNumber == nil || *q.DriverNumber != 44 {
		t.Fatalf("status %d, query %+v", rec.Code, q)
	}
	if q.Window.From == nil || !q.Window.From.Equal(time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC)) {
		t.Fatalf("fromUtc = %v", q.Window.From)
	}
	if q.Window.To == nil || !q.Window.To.Equal(time.Date(2026, 3, 8, 15, 10, 0, 500_000_000, time.UTC)) {
		t.Fatalf("toUtc = %v", q.Window.To)
	}
}

// TestRaceReplayStreamController_GetReplay_InvalidQuery proves an invalid driverNumber, fromUtc
// or toUtc is a 400 before the usecase runs, and an inverted window reported by the usecase is
// a 400 too.
func TestRaceReplayStreamController_GetReplay_InvalidQuery(t *testing.T) {
	for _, query := range []string{"?driverNumber=abc", "?driverNumber=0", "?fromUtc=yesterday", "?toUtc=2026-03-08"} {
		uc := &fakeRaceReplayUseCase{sessionExists: true}
		rec := httptest.NewRecorder()
		NewRaceReplayStreamController(uc).GetReplay(rec, newReplayRequestWithQuery("s1", query))
		if rec.Code != http.StatusBadRequest || uc.called {
			t.Fatalf("%s: status = %d, usecase called=%v; want 400 before the usecase", query, rec.Code, uc.called)
		}
	}

	rec := httptest.NewRecorder()
	NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExists: true, err: domain.ErrInvalidTimeWindow}).GetReplay(rec, newReplayRequest("s1"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("inverted window: status = %d, want 400", rec.Code)
	}
}

// TestRaceReplayStreamController_GetReplay_UnknownSession proves an unknown session yields 404
// before the replay is built.
func TestRaceReplayStreamController_GetReplay_UnknownSession(t *testing.T) {
	uc := &fakeRaceReplayUseCase{sessionExists: false}
	rec := httptest.NewRecorder()
	NewRaceReplayStreamController(uc).GetReplay(rec, newReplayRequest("missing"))
	if rec.Code != http.StatusNotFound || uc.called {
		t.Fatalf("status = %d, called=%v; want 404 without building", rec.Code, uc.called)
	}
}

// TestRaceReplayStreamController_GetReplay_UpstreamFailure proves a failure to reach
// championship-service while checking the session is a 502.
func TestRaceReplayStreamController_GetReplay_UpstreamFailure(t *testing.T) {
	rec := httptest.NewRecorder()
	NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExistsErr: errors.New("down")}).GetReplay(rec, newReplayRequest("s1"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

// TestRaceReplayStreamController_GetReplay_Busy proves the concurrent replay cap answers 503
// with a Retry-After header.
func TestRaceReplayStreamController_GetReplay_Busy(t *testing.T) {
	rec := httptest.NewRecorder()
	NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExists: true, err: domain.ErrReplayBusy}).GetReplay(rec, newReplayRequest("s1"))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != strconv.Itoa(replayRetryAfterSeconds) {
		t.Fatalf("status = %d Retry-After=%q, want 503 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
}

// TestRaceReplayStreamController_GetReplay_FailureBeforeFirstPart proves a usecase failure before
// any part is a normal 500 error envelope.
func TestRaceReplayStreamController_GetReplay_FailureBeforeFirstPart(t *testing.T) {
	rec := httptest.NewRecorder()
	NewRaceReplayStreamController(&fakeRaceReplayUseCase{sessionExists: true, err: errors.New("db down")}).GetReplay(rec, newReplayRequest("s1"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// TestRaceReplayStreamController_GetReplay_FailureMidStreamAborts proves a failure after the
// response started aborts the connection: the client cannot read a complete body, so a truncated
// dump never passes for a whole one.
func TestRaceReplayStreamController_GetReplay_FailureMidStreamAborts(t *testing.T) {
	big := make([]domain.RaceReplayTelemetrySample, 5000)
	uc := &fakeRaceReplayUseCase{
		sessionExists: true,
		parts:         []domain.ReplayPart{driverPart(t, "telemetry", 1, big)},
		err:           errors.New("db down"),
	}
	controller := NewRaceReplayStreamController(uc)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("sessionId", "s1")
		controller.GetReplay(w, r)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/sessions/s1/race/replay")
	if err != nil {
		return // aborted before the headers: also not a complete dump
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr == nil && json.Valid(body) {
		t.Fatalf("client read a complete JSON body (%d bytes) from a failed replay", len(body))
	}
}
