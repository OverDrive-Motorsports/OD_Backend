/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## session_controller_extra_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
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

// configurableSessionQueryUseCase implements ports.SessionQueryUseCase with a per-method result/
// error pair, used by tests below that need to exercise handlers other than GetSessionDataset
// (already covered by fakeSessionQueryUseCase in session_controller_test.go).
type configurableSessionQueryUseCase struct {
	listChampionshipsResult any
	err                     error

	getSessionResult          map[string]any
	listSessionDriversResult  map[string]any
	listSessionTeamsResult    map[string]any
	raceStandingsResult       any
	broadcastResult           map[string]any
	factsResult               map[string]any
	driverProfileResult       map[string]any
	driverBroadcastResult     map[string]any
	driverDatasetResult       map[string]any
	driverLapLocationResult   map[string]any
}

func (f *configurableSessionQueryUseCase) ListChampionships(ctx context.Context) (any, error) {
	return f.listChampionshipsResult, f.err
}
func (f *configurableSessionQueryUseCase) ListChampionshipEvents(ctx context.Context, code string) (any, error) {
	return f.listChampionshipsResult, f.err
}
func (f *configurableSessionQueryUseCase) GetEvent(ctx context.Context, eventID string) (any, error) {
	return f.listChampionshipsResult, f.err
}
func (f *configurableSessionQueryUseCase) ListEventSessions(ctx context.Context, eventID string) (any, error) {
	return f.listChampionshipsResult, f.err
}
func (f *configurableSessionQueryUseCase) GetSession(ctx context.Context, sessionID string) (map[string]any, error) {
	return f.getSessionResult, f.err
}
func (f *configurableSessionQueryUseCase) GetSessionMetadata(ctx context.Context, sessionID string) (map[string]any, error) {
	return f.getSessionResult, f.err
}
func (f *configurableSessionQueryUseCase) ListSessionDrivers(ctx context.Context, sessionID string) (map[string]any, error) {
	return f.listSessionDriversResult, f.err
}
func (f *configurableSessionQueryUseCase) ListSessionTeams(ctx context.Context, sessionID string) (map[string]any, error) {
	return f.listSessionTeamsResult, f.err
}
func (f *configurableSessionQueryUseCase) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (map[string]any, error) {
	return f.getSessionResult, f.err
}
func (f *configurableSessionQueryUseCase) GetSessionRaceStandings(ctx context.Context, sessionID string) (any, error) {
	return f.raceStandingsResult, f.err
}
func (f *configurableSessionQueryUseCase) GetSessionBroadcast(ctx context.Context, sessionID string) (map[string]any, error) {
	return f.broadcastResult, f.err
}
func (f *configurableSessionQueryUseCase) GetDriverProfile(ctx context.Context, sessionID string, driverNumber int) (map[string]any, error) {
	return f.driverProfileResult, f.err
}
func (f *configurableSessionQueryUseCase) GetDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (map[string]any, error) {
	return f.driverBroadcastResult, f.err
}
func (f *configurableSessionQueryUseCase) GetDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) (map[string]any, error) {
	return f.driverDatasetResult, f.err
}
func (f *configurableSessionQueryUseCase) GetDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (map[string]any, error) {
	return f.driverLapLocationResult, f.err
}
func (f *configurableSessionQueryUseCase) GetSessionFacts(ctx context.Context, sessionID string) (map[string]any, error) {
	return f.factsResult, f.err
}

func withSessionPath(method, path, sessionID string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.SetPathValue("sessionId", sessionID)
	return req
}

// TestSessionController_GetSession_NormalAndNotFound proves GetSession returns 200 with the
// mapped payload, and 404 when the usecase yields nil.
func TestSessionController_GetSession_NormalAndNotFound(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{getSessionResult: map[string]any{"id": "s1"}})
		rec := httptest.NewRecorder()
		c.GetSession(rec, withSessionPath(http.MethodGet, "/sessions/s1", "s1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{getSessionResult: nil})
		rec := httptest.NewRecorder()
		c.GetSession(rec, withSessionPath(http.MethodGet, "/sessions/missing", "missing"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("upstream failure -> 502", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{err: errors.New("down")})
		rec := httptest.NewRecorder()
		c.GetSession(rec, withSessionPath(http.MethodGet, "/sessions/s1", "s1"))
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestSessionController_ListChampionships_NormalCase proves the collection endpoints (which have
// no not-found branch) return 200 with the usecase's payload.
func TestSessionController_ListChampionships_NormalCase(t *testing.T) {
	c := NewSessionController(&configurableSessionQueryUseCase{listChampionshipsResult: []map[string]any{{"code": "f1"}}})
	req := httptest.NewRequest(http.MethodGet, "/championships", nil)
	rec := httptest.NewRecorder()
	c.ListChampionships(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestSessionController_ListChampionshipEvents_NotFound proves a nil payload from the usecase maps
// to 404 (unlike ListChampionships, which has no not-found branch).
func TestSessionController_ListChampionshipEvents_NotFound(t *testing.T) {
	c := NewSessionController(&configurableSessionQueryUseCase{listChampionshipsResult: nil})
	req := httptest.NewRequest(http.MethodGet, "/championships/f1/events", nil)
	req.SetPathValue("code", "f1")
	rec := httptest.NewRecorder()
	c.ListChampionshipEvents(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestSessionController_GetEvent_NotFound proves GetEvent maps a nil usecase payload to a 404
// EVENT error.
func TestSessionController_GetEvent_NotFound(t *testing.T) {
	c := NewSessionController(&configurableSessionQueryUseCase{listChampionshipsResult: nil})
	req := httptest.NewRequest(http.MethodGet, "/events/e1", nil)
	req.SetPathValue("eventId", "e1")
	rec := httptest.NewRecorder()
	c.GetEvent(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestSessionController_ListEventSessions_NormalCase proves the happy path returns 200.
func TestSessionController_ListEventSessions_NormalCase(t *testing.T) {
	c := NewSessionController(&configurableSessionQueryUseCase{listChampionshipsResult: []map[string]any{{"sessionId": "s1"}}})
	req := httptest.NewRequest(http.MethodGet, "/events/e1/sessions", nil)
	req.SetPathValue("eventId", "e1")
	rec := httptest.NewRecorder()
	c.ListEventSessions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestSessionController_ListSessionDrivers_NormalAndNotFound covers both branches.
func TestSessionController_ListSessionDrivers_NormalAndNotFound(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{listSessionDriversResult: map[string]any{"count": 1}})
		rec := httptest.NewRecorder()
		c.ListSessionDrivers(rec, withSessionPath(http.MethodGet, "/sessions/s1/drivers", "s1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("not found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{listSessionDriversResult: nil})
		rec := httptest.NewRecorder()
		c.ListSessionDrivers(rec, withSessionPath(http.MethodGet, "/sessions/missing/drivers", "missing"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestSessionController_ListSessionTeams_NormalCase mirrors ListSessionDrivers's happy path.
func TestSessionController_ListSessionTeams_NormalCase(t *testing.T) {
	c := NewSessionController(&configurableSessionQueryUseCase{listSessionTeamsResult: map[string]any{"count": 1}})
	rec := httptest.NewRecorder()
	c.ListSessionTeams(rec, withSessionPath(http.MethodGet, "/sessions/s1/teams", "s1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestSessionController_GetSessionRaceStandings_NormalCase proves the happy path returns 200.
func TestSessionController_GetSessionRaceStandings_NormalCase(t *testing.T) {
	c := NewSessionController(&configurableSessionQueryUseCase{raceStandingsResult: map[string]any{"leader": "VER"}})
	rec := httptest.NewRecorder()
	c.GetSessionRaceStandings(rec, withSessionPath(http.MethodGet, "/sessions/s1/standings/race", "s1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestSessionController_GetSessionBroadcast_NormalCase proves the happy path returns 200.
func TestSessionController_GetSessionBroadcast_NormalCase(t *testing.T) {
	c := NewSessionController(&configurableSessionQueryUseCase{broadcastResult: map[string]any{"broadcastUrl": "https://x"}})
	rec := httptest.NewRecorder()
	c.GetSessionBroadcast(rec, withSessionPath(http.MethodGet, "/sessions/s1/broadcast", "s1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestSessionController_GetSessionWeather_NormalAndErrorMapping proves GetSessionWeather calls
// GetSessionDataset with "weather" and maps a failure to a plain apierror.Internal (unlike
// GetSessionDataset itself, which classifies domain.ErrUnknownDataset specially - weather is a
// fixed dataset name here, so that branch does not apply).
func TestSessionController_GetSessionWeather_NormalAndErrorMapping(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{getSessionResult: map[string]any{"data": []any{}}})
		rec := httptest.NewRecorder()
		c.GetSessionWeather(rec, withSessionPath(http.MethodGet, "/sessions/s1/weather", "s1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("failure -> 500", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{err: errors.New("db down")})
		rec := httptest.NewRecorder()
		c.GetSessionWeather(rec, withSessionPath(http.MethodGet, "/sessions/s1/weather", "s1"))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestSessionController_GetSessionFacts_NormalAndNotFound covers both branches.
func TestSessionController_GetSessionFacts_NormalAndNotFound(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{factsResult: map[string]any{"count": 0}})
		rec := httptest.NewRecorder()
		c.GetSessionFacts(rec, withSessionPath(http.MethodGet, "/sessions/s1/facts", "s1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("not found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{factsResult: nil})
		rec := httptest.NewRecorder()
		c.GetSessionFacts(rec, withSessionPath(http.MethodGet, "/sessions/missing/facts", "missing"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

func withDriverPath(method, path, sessionID, driverNumber string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.SetPathValue("sessionId", sessionID)
	req.SetPathValue("driverNumber", driverNumber)
	return req
}

// TestSessionController_GetDriverProfile covers the normal case, not-found, and the invalid
// driverNumber path parameter (400, before the usecase is called).
func TestSessionController_GetDriverProfile(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{driverProfileResult: map[string]any{"driverName": "Max"}})
		rec := httptest.NewRecorder()
		c.GetDriverProfile(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/1", "s1", "1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("not found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{driverProfileResult: nil})
		rec := httptest.NewRecorder()
		c.GetDriverProfile(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/999", "s1", "999"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid driverNumber -> 400", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{})
		rec := httptest.NewRecorder()
		c.GetDriverProfile(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/abc", "s1", "abc"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestSessionController_GetDriverBroadcast covers the normal case, not-found, and invalid
// driverNumber.
func TestSessionController_GetDriverBroadcast(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{driverBroadcastResult: map[string]any{"broadcastUrl": "https://x"}})
		rec := httptest.NewRecorder()
		c.GetDriverBroadcast(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/1/broadcast", "s1", "1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("not found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{driverBroadcastResult: nil})
		rec := httptest.NewRecorder()
		c.GetDriverBroadcast(rec, withDriverPath(http.MethodGet, "/sessions/missing/drivers/1/broadcast", "missing", "1"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid driverNumber -> 400", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{})
		rec := httptest.NewRecorder()
		c.GetDriverBroadcast(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/abc/broadcast", "s1", "abc"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestSessionController_GetDriverDataset covers the normal case, not-found, invalid driverNumber,
// and the same classifyDatasetErr 400-vs-500 split already exercised for GetSessionDataset in
// session_controller_test.go.
func TestSessionController_GetDriverDataset(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{driverDatasetResult: map[string]any{"data": []any{}}})
		rec := httptest.NewRecorder()
		c.GetDriverDataset(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/1/datasets/speed", "s1", "1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("not found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{driverDatasetResult: nil})
		rec := httptest.NewRecorder()
		c.GetDriverDataset(rec, withDriverPath(http.MethodGet, "/sessions/missing/drivers/1/datasets/speed", "missing", "1"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid driverNumber -> 400", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{})
		rec := httptest.NewRecorder()
		c.GetDriverDataset(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/abc/datasets/speed", "s1", "abc"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("unknown dataset -> 400 via classifyDatasetErr", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{err: domain.ErrUnknownDataset})
		rec := httptest.NewRecorder()
		c.GetDriverDataset(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/1/datasets/bogus", "s1", "1"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestSessionController_GetDriverLapLocation covers the normal case, not-found, and both invalid
// path parameters (driverNumber, lapNumber).
func TestSessionController_GetDriverLapLocation(t *testing.T) {
	withLapPath := func(sessionID, driverNumber, lapNumber string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID+"/drivers/"+driverNumber+"/laps/"+lapNumber+"/location", nil)
		req.SetPathValue("sessionId", sessionID)
		req.SetPathValue("driverNumber", driverNumber)
		req.SetPathValue("lapNumber", lapNumber)
		return req
	}

	t.Run("found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{driverLapLocationResult: map[string]any{"data": []any{}}})
		rec := httptest.NewRecorder()
		c.GetDriverLapLocation(rec, withLapPath("s1", "1", "3"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("not found", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{driverLapLocationResult: nil})
		rec := httptest.NewRecorder()
		c.GetDriverLapLocation(rec, withLapPath("missing", "1", "3"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid driverNumber -> 400", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{})
		rec := httptest.NewRecorder()
		c.GetDriverLapLocation(rec, withLapPath("s1", "abc", "3"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid lapNumber -> 400", func(t *testing.T) {
		c := NewSessionController(&configurableSessionQueryUseCase{})
		rec := httptest.NewRecorder()
		c.GetDriverLapLocation(rec, withLapPath("s1", "1", "abc"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}
