/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_query_test.go - Package usecases source file for services/race-data-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"errors"
	"testing"

	"overdrive/services/race-data-service/src/core/ports"
)

// fakeChampionshipClientForSessionQuery implements ports.ChampionshipClient in full, since
// SessionQueryUseCase is the one consumer that exercises nearly every method on the interface.
type fakeChampionshipClientForSessionQuery struct {
	championships     any
	championshipsErr  error
	events            any
	eventsErr         error
	eventPayload      any
	eventPayloadErr   error
	eventSessions     any
	eventSessionsErr  error
	session           *ports.ChampionshipSessionRef
	sessionErr        error
	event             *ports.ChampionshipEventRef
	eventErr          error
	drivers           []ports.ChampionshipDriverRef
	driversErr        error
	teams             []map[string]any
	teamsErr          error
	sessionDataset    map[string]any
	sessionDatasetErr error
	raceStandings     any
	raceStandingsErr  error
	broadcast         map[string]any
	broadcastErr      error
}

func (f *fakeChampionshipClientForSessionQuery) ListChampionships(ctx context.Context) (any, error) {
	return f.championships, f.championshipsErr
}
func (f *fakeChampionshipClientForSessionQuery) ListChampionshipEvents(ctx context.Context, code string) (any, error) {
	return f.events, f.eventsErr
}
func (f *fakeChampionshipClientForSessionQuery) GetEventPayload(ctx context.Context, eventID string) (any, error) {
	return f.eventPayload, f.eventPayloadErr
}
func (f *fakeChampionshipClientForSessionQuery) ListEventSessions(ctx context.Context, eventID string) (any, error) {
	return f.eventSessions, f.eventSessionsErr
}
func (f *fakeChampionshipClientForSessionQuery) GetSession(ctx context.Context, sessionID string) (*ports.ChampionshipSessionRef, error) {
	return f.session, f.sessionErr
}
func (f *fakeChampionshipClientForSessionQuery) GetEvent(ctx context.Context, eventID string) (*ports.ChampionshipEventRef, error) {
	return f.event, f.eventErr
}
func (f *fakeChampionshipClientForSessionQuery) ListSessionDrivers(ctx context.Context, sessionID string) ([]ports.ChampionshipDriverRef, error) {
	return f.drivers, f.driversErr
}
func (f *fakeChampionshipClientForSessionQuery) ListSessionTeams(ctx context.Context, sessionID string) ([]map[string]any, error) {
	return f.teams, f.teamsErr
}
func (f *fakeChampionshipClientForSessionQuery) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (map[string]any, error) {
	return f.sessionDataset, f.sessionDatasetErr
}
func (f *fakeChampionshipClientForSessionQuery) GetSessionRaceStandings(ctx context.Context, sessionID string) (any, error) {
	return f.raceStandings, f.raceStandingsErr
}
func (f *fakeChampionshipClientForSessionQuery) GetSessionBroadcast(ctx context.Context, sessionID string) (map[string]any, error) {
	return f.broadcast, f.broadcastErr
}

// fakeSessionQueryRepository implements ports.SessionQueryRepository, keyed by dataset name so a
// single fake can serve the multi-dataset aggregation in GetSessionFacts.
type fakeSessionQueryRepository struct {
	datasetsBySessionDataset map[string][]map[string]any
	datasetErr               error
	driverDataset            []map[string]any
	driverDatasetErr         error
	lapLocation              map[string]any
	lapLocationErr           error
	broadcastURL             string
	broadcastErr             error
}

func (f *fakeSessionQueryRepository) GetSessionDataset(ctx context.Context, sessionID string, dataset string) ([]map[string]any, error) {
	if f.datasetErr != nil {
		return nil, f.datasetErr
	}
	return f.datasetsBySessionDataset[dataset], nil
}
func (f *fakeSessionQueryRepository) GetDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) ([]map[string]any, error) {
	return f.driverDataset, f.driverDatasetErr
}
func (f *fakeSessionQueryRepository) GetDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (map[string]any, error) {
	return f.lapLocation, f.lapLocationErr
}
func (f *fakeSessionQueryRepository) GetDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (string, error) {
	return f.broadcastURL, f.broadcastErr
}

func sq(client *fakeChampionshipClientForSessionQuery, repo *fakeSessionQueryRepository) *SessionQueryUseCase {
	return NewSessionQueryUseCase(repo, client)
}

// TestSessionQueryUseCase_ThinPassThroughs proves ListChampionships/ListChampionshipEvents/
// GetEvent/ListEventSessions/GetSessionRaceStandings/GetSessionBroadcast are unmodified
// pass-throughs to the championship-service client.
func TestSessionQueryUseCase_ThinPassThroughs(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		championships: []map[string]any{{"code": "f1"}},
		events:        []map[string]any{{"eventId": "e1"}},
		eventPayload:  map[string]any{"eventId": "e1"},
		eventSessions: []map[string]any{{"sessionId": "s1"}},
		raceStandings: map[string]any{"leader": "VER"},
		broadcast:     map[string]any{"broadcastUrl": "https://x"},
	}
	uc := sq(client, &fakeSessionQueryRepository{})

	if got, err := uc.ListChampionships(context.Background()); err != nil || got == nil {
		t.Fatalf("ListChampionships: got %v err %v", got, err)
	}
	if got, err := uc.ListChampionshipEvents(context.Background(), "f1"); err != nil || got == nil {
		t.Fatalf("ListChampionshipEvents: got %v err %v", got, err)
	}
	if got, err := uc.GetEvent(context.Background(), "e1"); err != nil || got == nil {
		t.Fatalf("GetEvent: got %v err %v", got, err)
	}
	if got, err := uc.ListEventSessions(context.Background(), "e1"); err != nil || got == nil {
		t.Fatalf("ListEventSessions: got %v err %v", got, err)
	}
	if got, err := uc.GetSessionRaceStandings(context.Background(), "s1"); err != nil || got == nil {
		t.Fatalf("GetSessionRaceStandings: got %v err %v", got, err)
	}
	if got, err := uc.GetSessionBroadcast(context.Background(), "s1"); err != nil || got == nil {
		t.Fatalf("GetSessionBroadcast: got %v err %v", got, err)
	}

	boom := errors.New("downstream boom")
	client.championshipsErr = boom
	if _, err := uc.ListChampionships(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("expected downstream error to propagate, got %v", err)
	}
}

// TestSessionQueryUseCase_GetSession proves GetSession maps a championship-service session
// reference into the snake_case response map, returns nil for an unknown session, and propagates
// downstream failures untouched.
func TestSessionQueryUseCase_GetSession(t *testing.T) {
	t.Run("known session maps fields", func(t *testing.T) {
		client := &fakeChampionshipClientForSessionQuery{session: &ports.ChampionshipSessionRef{
			ID: "s1", EventID: "e1", Type: "race", Status: "finished", Name: "Race",
		}}
		uc := sq(client, &fakeSessionQueryRepository{})
		got, err := uc.GetSession(context.Background(), "s1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["id"] != "s1" || got["event_id"] != "e1" || got["name"] != "Race" {
			t.Fatalf("expected mapped session fields, got %+v", got)
		}
	})

	t.Run("unknown session returns nil, nil", func(t *testing.T) {
		uc := sq(&fakeChampionshipClientForSessionQuery{session: nil}, &fakeSessionQueryRepository{})
		got, err := uc.GetSession(context.Background(), "missing")
		if err != nil || got != nil {
			t.Fatalf("expected nil, nil got %v %v", got, err)
		}
	})

	t.Run("downstream error propagates", func(t *testing.T) {
		boom := errors.New("boom")
		uc := sq(&fakeChampionshipClientForSessionQuery{sessionErr: boom}, &fakeSessionQueryRepository{})
		_, err := uc.GetSession(context.Background(), "s1")
		if !errors.Is(err, boom) {
			t.Fatalf("expected error to propagate, got %v", err)
		}
	})
}

// TestSessionQueryUseCase_GetSessionMetadata proves it combines the session and its event into a
// metadata payload, and returns nil when the session doesn't exist (via loadContext).
func TestSessionQueryUseCase_GetSessionMetadata(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		session: &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1", Name: "Race"},
		event:   &ports.ChampionshipEventRef{ID: "e1", Name: "Monaco GP", SeasonYear: 2026},
	}
	uc := sq(client, &fakeSessionQueryRepository{})

	got, err := uc.GetSessionMetadata(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["session_id"] != "s1" {
		t.Fatalf("expected session_id=s1, got %+v", got)
	}
	metadata, ok := got["metadata"].(map[string]any)
	if !ok || metadata["meeting_name"] != "Monaco GP" {
		t.Fatalf("expected enriched metadata, got %+v", got["metadata"])
	}

	uc2 := sq(&fakeChampionshipClientForSessionQuery{session: nil}, &fakeSessionQueryRepository{})
	got2, err := uc2.GetSessionMetadata(context.Background(), "missing")
	if err != nil || got2 != nil {
		t.Fatalf("expected nil, nil for unknown session, got %v %v", got2, err)
	}
}

// TestSessionQueryUseCase_ListSessionDrivers proves the {count,data,session_id} envelope is built
// from championship-service's driver list, and that a nil driver list (unknown session) surfaces
// as nil, nil rather than an empty envelope.
func TestSessionQueryUseCase_ListSessionDrivers(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{drivers: []ports.ChampionshipDriverRef{
		{DriverNumber: 1, DriverName: "Max", TeamName: "Red Bull", TeamColor: "#0600EF"},
	}}
	uc := sq(client, &fakeSessionQueryRepository{})

	got, err := uc.ListSessionDrivers(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["count"] != 1 || got["session_id"] != "s1" {
		t.Fatalf("expected envelope with count=1, got %+v", got)
	}

	uc2 := sq(&fakeChampionshipClientForSessionQuery{drivers: nil}, &fakeSessionQueryRepository{})
	got2, err := uc2.ListSessionDrivers(context.Background(), "missing")
	if err != nil || got2 != nil {
		t.Fatalf("expected nil, nil for unknown session, got %v %v", got2, err)
	}
}

// TestSessionQueryUseCase_ListSessionTeams mirrors ListSessionDrivers's envelope/nil-handling for
// the teams pass-through.
func TestSessionQueryUseCase_ListSessionTeams(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{teams: []map[string]any{{"teamId": "t1"}}}
	uc := sq(client, &fakeSessionQueryRepository{})

	got, err := uc.ListSessionTeams(context.Background(), "s1")
	if err != nil || got["count"] != 1 {
		t.Fatalf("expected envelope with count=1, got %+v err %v", got, err)
	}

	uc2 := sq(&fakeChampionshipClientForSessionQuery{teams: nil}, &fakeSessionQueryRepository{})
	got2, err := uc2.ListSessionTeams(context.Background(), "missing")
	if err != nil || got2 != nil {
		t.Fatalf("expected nil, nil for unknown session, got %v %v", got2, err)
	}
}

// TestSessionQueryUseCase_GetSessionDataset_RemoteDataset proves a dataset in the remote allow-list
// (e.g. "starting_grid") is proxied to championship-service instead of the local repository.
func TestSessionQueryUseCase_GetSessionDataset_RemoteDataset(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		session:        &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1"},
		event:          &ports.ChampionshipEventRef{ID: "e1"},
		sessionDataset: map[string]any{"dataset": "starting_grid"},
	}
	repo := &fakeSessionQueryRepository{}
	uc := sq(client, repo)

	got, err := uc.GetSessionDataset(context.Background(), "s1", "starting_grid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["dataset"] != "starting_grid" {
		t.Fatalf("expected the remote payload to be returned unmodified, got %+v", got)
	}
}

// TestSessionQueryUseCase_GetSessionDataset_LocalDataset proves a local dataset is read from the
// repository, enriched with driver metadata via driverLookup, and wrapped with session metadata.
func TestSessionQueryUseCase_GetSessionDataset_LocalDataset(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		session: &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1"},
		event:   &ports.ChampionshipEventRef{ID: "e1", Name: "Monaco GP"},
		drivers: []ports.ChampionshipDriverRef{{DriverNumber: 1, DriverName: "Max", TeamName: "Red Bull"}},
	}
	repo := &fakeSessionQueryRepository{datasetsBySessionDataset: map[string][]map[string]any{
		"laps": {{"driver_number": 1, "lap_number": 3}},
	}}
	uc := sq(client, repo)

	got, err := uc.GetSessionDataset(context.Background(), "s1", "laps")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["count"] != 1 {
		t.Fatalf("expected count=1, got %+v", got)
	}
	rows, ok := got["data"].([]map[string]any)
	if !ok || rows[0]["driver_name"] != "Max" {
		t.Fatalf("expected the row to be enriched with driver metadata, got %+v", got["data"])
	}
}

// TestSessionQueryUseCase_GetSessionDataset_UnknownSession proves an unknown session short-circuits
// before either the remote or local dataset path is taken.
func TestSessionQueryUseCase_GetSessionDataset_UnknownSession(t *testing.T) {
	uc := sq(&fakeChampionshipClientForSessionQuery{session: nil}, &fakeSessionQueryRepository{})
	got, err := uc.GetSessionDataset(context.Background(), "missing", "laps")
	if err != nil || got != nil {
		t.Fatalf("expected nil, nil got %v %v", got, err)
	}
}

// TestSessionQueryUseCase_GetDriverProfile proves it finds the driver by number in the
// championship-service driver list and enriches it with a broadcast URL from the local repository,
// or returns nil when the driver number is not present in the session, or the session itself
// doesn't exist.
func TestSessionQueryUseCase_GetDriverProfile(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{drivers: []ports.ChampionshipDriverRef{
		{DriverNumber: 44, DriverName: "Lewis", TeamName: "Mercedes"},
	}}
	repo := &fakeSessionQueryRepository{broadcastURL: "https://stream/44"}
	uc := sq(client, repo)

	got, err := uc.GetDriverProfile(context.Background(), "s1", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["driver_name"] != "Lewis" || got["broadcast_url"] != "https://stream/44" {
		t.Fatalf("expected enriched driver profile, got %+v", got)
	}

	got2, err := uc.GetDriverProfile(context.Background(), "s1", 999)
	if err != nil || got2 != nil {
		t.Fatalf("expected nil for a driver not in this session, got %v %v", got2, err)
	}

	uc2 := sq(&fakeChampionshipClientForSessionQuery{drivers: nil}, &fakeSessionQueryRepository{})
	got3, err := uc2.GetDriverProfile(context.Background(), "missing", 44)
	if err != nil || got3 != nil {
		t.Fatalf("expected nil for unknown session, got %v %v", got3, err)
	}
}

// TestSessionQueryUseCase_GetDriverBroadcast proves it loads session context, then the driver's
// broadcast URL from the local repository, returning nil when the session doesn't exist.
func TestSessionQueryUseCase_GetDriverBroadcast(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{session: &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1"}}
	repo := &fakeSessionQueryRepository{broadcastURL: "https://stream/1"}
	uc := sq(client, repo)

	got, err := uc.GetDriverBroadcast(context.Background(), "s1", 1)
	if err != nil || got["broadcast_url"] != "https://stream/1" {
		t.Fatalf("expected broadcast_url, got %+v err %v", got, err)
	}

	uc2 := sq(&fakeChampionshipClientForSessionQuery{session: nil}, &fakeSessionQueryRepository{})
	got2, err := uc2.GetDriverBroadcast(context.Background(), "missing", 1)
	if err != nil || got2 != nil {
		t.Fatalf("expected nil for unknown session, got %v %v", got2, err)
	}
}

// TestSessionQueryUseCase_GetDriverDataset_ResultSpecialCase proves the "result" dataset name is a
// special case that proxies to championship-service's "session_result" dataset and filters the
// returned rows down to just the requested driver, rather than reading from the local repository.
func TestSessionQueryUseCase_GetDriverDataset_ResultSpecialCase(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		session: &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1"},
		event:   &ports.ChampionshipEventRef{ID: "e1"},
		sessionDataset: map[string]any{
			"data": []any{
				map[string]any{"driver_number": 1, "position": 1},
				map[string]any{"driver_number": 44, "position": 2},
			},
		},
	}
	uc := sq(client, &fakeSessionQueryRepository{})

	got, err := uc.GetDriverDataset(context.Background(), "s1", 44, "result")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["count"] != 1 || got["driver_number"] != 44 {
		t.Fatalf("expected filtered result for driver 44, got %+v", got)
	}
	rows, ok := got["data"].([]map[string]any)
	if !ok || len(rows) != 1 || rows[0]["position"] != 2 {
		t.Fatalf("expected only driver 44's row, got %+v", got["data"])
	}
}

// TestSessionQueryUseCase_GetDriverDataset_ResultSpecialCase_NilUpstream proves that when
// championship-service has no session_result payload at all (nil, not an error), GetDriverDataset
// still returns a well-formed empty-filtered envelope instead of propagating the nil as a
// not-found.
func TestSessionQueryUseCase_GetDriverDataset_ResultSpecialCase_NilUpstream(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		session:        &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1"},
		event:          &ports.ChampionshipEventRef{ID: "e1"},
		sessionDataset: nil,
	}
	uc := sq(client, &fakeSessionQueryRepository{})

	got, err := uc.GetDriverDataset(context.Background(), "s1", 44, "result")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["count"] != 0 || got["driver_number"] != 44 {
		t.Fatalf("expected an empty-filtered envelope, got %+v", got)
	}
}

// TestSessionQueryUseCase_GetDriverDataset_LocalDataset proves a non-"result" dataset reads from
// the local repository and is enriched the same way GetSessionDataset's local path is.
func TestSessionQueryUseCase_GetDriverDataset_LocalDataset(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		session: &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1"},
		event:   &ports.ChampionshipEventRef{ID: "e1"},
		drivers: []ports.ChampionshipDriverRef{{DriverNumber: 44, DriverName: "Lewis"}},
	}
	repo := &fakeSessionQueryRepository{driverDataset: []map[string]any{{"driver_number": 44, "speed": 300}}}
	uc := sq(client, repo)

	got, err := uc.GetDriverDataset(context.Background(), "s1", 44, "speed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows, ok := got["data"].([]map[string]any)
	if !ok || rows[0]["driver_name"] != "Lewis" {
		t.Fatalf("expected the row to be enriched, got %+v", got["data"])
	}
}

// TestSessionQueryUseCase_GetSessionFacts proves it merges race_control/overtakes/pit/team_radio
// rows into a single sorted facts array, tagging each with fact_type and enriching driver-scoped
// rows (but not overtakes, which have no single driver_number field) with driver metadata.
func TestSessionQueryUseCase_GetSessionFacts(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		session: &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1"},
		event:   &ports.ChampionshipEventRef{ID: "e1"},
		drivers: []ports.ChampionshipDriverRef{{DriverNumber: 1, DriverName: "Max"}},
	}
	repo := &fakeSessionQueryRepository{datasetsBySessionDataset: map[string][]map[string]any{
		"race_control": {{"driver_number": 1, "date_utc": "2026-03-08T15:00:02Z", "message": "yellow flag"}},
		"overtakes":    {{"date_utc": "2026-03-08T15:00:01Z"}},
		"pit":          {{"driver_number": 1, "date_utc": "2026-03-08T15:00:03Z"}},
		"team_radio":   {{"driver_number": 1, "date_utc": "2026-03-08T15:00:00Z"}},
	}}
	uc := sq(client, repo)

	got, err := uc.GetSessionFacts(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["count"] != 4 {
		t.Fatalf("expected 4 merged facts, got %+v", got["count"])
	}
	facts, ok := got["data"].([]map[string]any)
	if !ok || len(facts) != 4 {
		t.Fatalf("expected a 4-element facts array, got %+v", got["data"])
	}
	// Sorted ascending by date: team_radio (00) < overtake (01) < race_control (02) < pit (03).
	if facts[0]["fact_type"] != "team_radio" || facts[3]["fact_type"] != "pit_stop" {
		t.Fatalf("expected facts sorted oldest-first by date_utc, got %+v", facts)
	}
	if facts[0]["driver_name"] != "Max" {
		t.Fatalf("expected the driver-scoped fact to be enriched, got %+v", facts[0])
	}
}

// TestSessionQueryUseCase_GetSessionFacts_UnknownSession proves it short-circuits on an unknown
// session before touching the repository at all.
func TestSessionQueryUseCase_GetSessionFacts_UnknownSession(t *testing.T) {
	uc := sq(&fakeChampionshipClientForSessionQuery{session: nil}, &fakeSessionQueryRepository{})
	got, err := uc.GetSessionFacts(context.Background(), "missing")
	if err != nil || got != nil {
		t.Fatalf("expected nil, nil got %v %v", got, err)
	}
}

// TestSessionQueryUseCase_GetDriverLapLocation proves both branches: a nil repository payload
// (no samples for that lap) produces a synthetic empty response rather than a bare nil, and a
// present payload has its rows enriched with driver metadata and gets session metadata attached.
func TestSessionQueryUseCase_GetDriverLapLocation(t *testing.T) {
	client := &fakeChampionshipClientForSessionQuery{
		session: &ports.ChampionshipSessionRef{ID: "s1", EventID: "e1"},
		event:   &ports.ChampionshipEventRef{ID: "e1"},
		drivers: []ports.ChampionshipDriverRef{{DriverNumber: 1, DriverName: "Max"}},
	}

	t.Run("no samples returns a synthetic empty payload, not nil", func(t *testing.T) {
		repo := &fakeSessionQueryRepository{lapLocation: nil}
		uc := sq(client, repo)
		got, err := uc.GetDriverLapLocation(context.Background(), "s1", 1, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["count"] != 0 || got["lap_number"] != 3 {
			t.Fatalf("expected an empty synthetic payload, got %+v", got)
		}
	})

	t.Run("present payload is enriched and gets metadata", func(t *testing.T) {
		repo := &fakeSessionQueryRepository{lapLocation: map[string]any{
			"data": []map[string]any{{"driver_number": 1, "x": 1.0}},
		}}
		uc := sq(client, repo)
		got, err := uc.GetDriverLapLocation(context.Background(), "s1", 1, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rows := got["data"].([]map[string]any)
		if rows[0]["driver_name"] != "Max" {
			t.Fatalf("expected the row to be enriched, got %+v", rows)
		}
		if _, ok := got["metadata"]; !ok {
			t.Fatalf("expected metadata to be attached, got %+v", got)
		}
	})

	t.Run("unknown session returns nil, nil", func(t *testing.T) {
		uc := sq(&fakeChampionshipClientForSessionQuery{session: nil}, &fakeSessionQueryRepository{})
		got, err := uc.GetDriverLapLocation(context.Background(), "missing", 1, 3)
		if err != nil || got != nil {
			t.Fatalf("expected nil, nil got %v %v", got, err)
		}
	})
}

// TestSessionMetadata_NilEventOrSession proves sessionMetadata falls back to a minimal
// {"provider":"openf1"} payload when either input is nil, instead of panicking on a nil deref.
func TestSessionMetadata_NilEventOrSession(t *testing.T) {
	got := sessionMetadata(nil, nil)
	if got["provider"] != "openf1" || len(got) != 1 {
		t.Fatalf("expected minimal fallback metadata, got %+v", got)
	}
}

// TestIsRemoteDataset proves the exact allow-list of datasets proxied to championship-service.
func TestIsRemoteDataset(t *testing.T) {
	remote := []string{"session_result", "starting_grid", "championship_drivers", "championship_teams"}
	for _, name := range remote {
		if !isRemoteDataset(name) {
			t.Fatalf("expected %q to be a remote dataset", name)
		}
	}
	if isRemoteDataset("laps") {
		t.Fatal("expected \"laps\" to be a local dataset")
	}
}

// TestIntFromAny_SessionQuery covers every supported dynamic-value branch of the session_query
// package's intFromAny, including the string-parsing and empty-string/unsupported-type fallbacks.
func TestIntFromAny_SessionQuery(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{"int", 5, 5},
		{"int32", int32(6), 6},
		{"int64", int64(7), 7},
		{"float64", float64(8.9), 8},
		{"numeric string", "9", 9},
		{"empty string", "", 0},
		{"unsupported type", true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := intFromAny(tc.value); got != tc.want {
				t.Fatalf("intFromAny(%#v) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

// TestStringifyFactDate proves it prefers "date_utc" over "date" and falls back to empty string
// when neither key is present, backing GetSessionFacts' chronological sort.
func TestStringifyFactDate(t *testing.T) {
	if got := stringifyFactDate(map[string]any{"date_utc": "2026-01-01T00:00:00Z", "date": "ignored"}); got != "2026-01-01T00:00:00Z" {
		t.Fatalf("expected date_utc to take priority, got %q", got)
	}
	if got := stringifyFactDate(map[string]any{"date": "2026-01-01T00:00:00Z"}); got != "2026-01-01T00:00:00Z" {
		t.Fatalf("expected fallback to date, got %q", got)
	}
	if got := stringifyFactDate(map[string]any{}); got != "" {
		t.Fatalf("expected empty string fallback, got %q", got)
	}
}

// TestEnrichDriverRow_UnknownDriverNumberOrMissingLookup proves enrichDriverRow is a no-op when
// the row has no usable driver_number, or when that driver number isn't in the lookup - it must
// never panic or corrupt the row in either case.
func TestEnrichDriverRow_UnknownDriverNumberOrMissingLookup(t *testing.T) {
	lookup := map[int]ports.ChampionshipDriverRef{1: {DriverNumber: 1, DriverName: "Max"}}

	row := map[string]any{"driver_number": 0}
	enrichDriverRow(row, lookup)
	if _, ok := row["driver_name"]; ok {
		t.Fatalf("expected no enrichment for a zero/missing driver_number, got %+v", row)
	}

	row2 := map[string]any{"driver_number": 999}
	enrichDriverRow(row2, lookup)
	if _, ok := row2["driver_name"]; ok {
		t.Fatalf("expected no enrichment for a driver not in the lookup, got %+v", row2)
	}

	row3 := map[string]any{"driver_number": 1, "driver_name": "already set"}
	enrichDriverRow(row3, lookup)
	if row3["driver_name"] != "already set" {
		t.Fatalf("expected an existing driver_name to be preserved, got %+v", row3)
	}
}
