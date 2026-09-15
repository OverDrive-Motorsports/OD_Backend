/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_live_test.go - Package usecases source file for services/race-data-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// fakeRaceLiveRepository is an in-memory ports.RaceLiveRepository used to exercise
// RaceLiveUseCase without a DB, in the style of fakeRaceStreamRepository in
// race_replay_stream_test.go.
type fakeRaceLiveRepository struct {
	positions     []domain.RacePosition
	driverNumbers []int
	lapsByDriver  map[int]domain.RaceLapsResponse
	stints        []domain.RaceStint
	pitStops      []domain.RacePitStop
	weather       []domain.RaceWeatherSample
	radio         []domain.RaceRadioMessage

	err error

	lastListPositionsDriver *int
	lastListRadioDriver     *int
}

func (f *fakeRaceLiveRepository) ListPositions(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) ([]domain.RacePosition, error) {
	f.lastListPositionsDriver = driverNumber
	if f.err != nil {
		return nil, f.err
	}
	if driverNumber == nil {
		return f.positions, nil
	}
	var out []domain.RacePosition
	for _, p := range f.positions {
		if p.DriverNumber == *driverNumber {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRaceLiveRepository) ListDriverNumbers(ctx context.Context, sessionID string) ([]int, error) {
	return f.driverNumbers, f.err
}

func (f *fakeRaceLiveRepository) ListLapsForDriver(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) (domain.RaceLapsResponse, error) {
	if f.err != nil {
		return domain.RaceLapsResponse{}, f.err
	}
	return f.lapsByDriver[driverNumber], nil
}

func (f *fakeRaceLiveRepository) ListStints(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceStint, error) {
	return f.stints, f.err
}

func (f *fakeRaceLiveRepository) ListPitStops(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePitStop, error) {
	return f.pitStops, f.err
}

func (f *fakeRaceLiveRepository) ListWeather(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error) {
	return f.weather, f.err
}

func (f *fakeRaceLiveRepository) ListRadio(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error) {
	f.lastListRadioDriver = driverNumber
	return f.radio, f.err
}

// fakeChampionshipClientForLive only implements GetSession, the only method RaceLiveUseCase
// (via SessionExists) actually calls.
type fakeChampionshipClientForLive struct {
	ports.ChampionshipClient
	session *ports.ChampionshipSessionRef
	err     error
}

func (f *fakeChampionshipClientForLive) GetSession(ctx context.Context, sessionID string) (*ports.ChampionshipSessionRef, error) {
	return f.session, f.err
}

// fakeRaceControlBroadcaster is a minimal ports.RaceControlBroadcaster fake used only to prove
// RaceLiveUseCase.WaitForRaceControl forwards to the broadcaster unmodified; the broadcaster's
// own behavior is exercised directly in race_control_broadcaster_test.go.
type fakeRaceControlBroadcaster struct {
	waitReturn        []domain.RaceControlEvent
	lastWaitSessionID string
	lastWaitTimeout   time.Duration
}

func (f *fakeRaceControlBroadcaster) Publish(sessionID string, events []domain.RaceControlEvent) {}

func (f *fakeRaceControlBroadcaster) Wait(ctx context.Context, sessionID string, timeout time.Duration) []domain.RaceControlEvent {
	f.lastWaitSessionID = sessionID
	f.lastWaitTimeout = timeout
	return f.waitReturn
}

func newRaceLiveUseCase(repo *fakeRaceLiveRepository, client ports.ChampionshipClient, broadcaster ports.RaceControlBroadcaster) *RaceLiveUseCase {
	return NewRaceLiveUseCase(repo, client, broadcaster)
}

func intPtr(v int) *int { return &v }

// TestRaceLiveUseCase_SessionExists proves SessionExists reflects championship-service's
// nil/non-nil session answer, which backs the "unknown sessionId -> 404" contract for every
// /race/* and /telemetry/* endpoint.
func TestRaceLiveUseCase_SessionExists(t *testing.T) {
	t.Run("known session", func(t *testing.T) {
		uc := newRaceLiveUseCase(&fakeRaceLiveRepository{}, &fakeChampionshipClientForLive{session: &ports.ChampionshipSessionRef{ID: "s1"}}, &fakeRaceControlBroadcaster{})
		exists, err := uc.SessionExists(context.Background(), "s1")
		if err != nil || !exists {
			t.Fatalf("expected exists=true, err=nil, got exists=%v err=%v", exists, err)
		}
	})

	t.Run("unknown session", func(t *testing.T) {
		uc := newRaceLiveUseCase(&fakeRaceLiveRepository{}, &fakeChampionshipClientForLive{session: nil}, &fakeRaceControlBroadcaster{})
		exists, err := uc.SessionExists(context.Background(), "does-not-exist")
		if err != nil || exists {
			t.Fatalf("expected exists=false, err=nil, got exists=%v err=%v", exists, err)
		}
	})

	t.Run("downstream failure propagates", func(t *testing.T) {
		boom := errors.New("championship-service unreachable")
		uc := newRaceLiveUseCase(&fakeRaceLiveRepository{}, &fakeChampionshipClientForLive{err: boom}, &fakeRaceControlBroadcaster{})
		_, err := uc.SessionExists(context.Background(), "s1")
		if !errors.Is(err, boom) {
			t.Fatalf("expected the downstream error to propagate, got %v", err)
		}
	})
}

// TestRaceLiveUseCase_GetPosition_ShapeBranch proves the object-vs-array shape switch: a single
// driverNumber narrows to one object (or nil if that driver has no rows), while an absent
// driverNumber returns the bare array of every driver.
func TestRaceLiveUseCase_GetPosition_ShapeBranch(t *testing.T) {
	repo := &fakeRaceLiveRepository{positions: []domain.RacePosition{
		{DriverNumber: 1, Position: 1},
		{DriverNumber: 44, Position: 2},
	}}
	uc := newRaceLiveUseCase(repo, &fakeChampionshipClientForLive{}, &fakeRaceControlBroadcaster{})

	t.Run("no driverNumber returns bare array", func(t *testing.T) {
		got, err := uc.GetPosition(context.Background(), "s1", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		items, ok := got.([]domain.RacePosition)
		if !ok || len(items) != 2 {
			t.Fatalf("expected a 2-element []domain.RacePosition, got %#v", got)
		}
	})

	t.Run("driverNumber given returns single object", func(t *testing.T) {
		got, err := uc.GetPosition(context.Background(), "s1", intPtr(44), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		item, ok := got.(domain.RacePosition)
		if !ok || item.DriverNumber != 44 {
			t.Fatalf("expected a single domain.RacePosition for driver 44, got %#v", got)
		}
	})

	t.Run("unknown driverNumber returns nil, not an empty array", func(t *testing.T) {
		got, err := uc.GetPosition(context.Background(), "s1", intPtr(999), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil for an unknown driver, got %#v", got)
		}
	})
}

// TestRaceLiveUseCase_GetLaps_ShapeBranch mirrors GetPosition's object-vs-array shape switch for
// /race/laps, additionally proving the "no driverNumber" branch fans out across every driver
// returned by ListDriverNumbers.
func TestRaceLiveUseCase_GetLaps_ShapeBranch(t *testing.T) {
	best := 78.5
	repo := &fakeRaceLiveRepository{
		driverNumbers: []int{1, 44},
		lapsByDriver: map[int]domain.RaceLapsResponse{
			1:  {DriverNumber: 1, Laps: []domain.RaceLapEntry{{LapNumber: 1}}, BestLap: &best},
			44: {DriverNumber: 44, Laps: []domain.RaceLapEntry{{LapNumber: 1}, {LapNumber: 2}}},
		},
	}
	uc := newRaceLiveUseCase(repo, &fakeChampionshipClientForLive{}, &fakeRaceControlBroadcaster{})

	t.Run("no driverNumber fans out across all drivers", func(t *testing.T) {
		got, err := uc.GetLaps(context.Background(), "s1", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		items, ok := got.([]domain.RaceLapsResponse)
		if !ok || len(items) != 2 {
			t.Fatalf("expected a 2-element []domain.RaceLapsResponse, got %#v", got)
		}
	})

	t.Run("driverNumber given returns single object", func(t *testing.T) {
		got, err := uc.GetLaps(context.Background(), "s1", intPtr(1), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		item, ok := got.(domain.RaceLapsResponse)
		if !ok || item.DriverNumber != 1 {
			t.Fatalf("expected a single domain.RaceLapsResponse for driver 1, got %#v", got)
		}
	})

	t.Run("unknown driverNumber with no laps and no bestLap returns nil", func(t *testing.T) {
		got, err := uc.GetLaps(context.Background(), "s1", intPtr(999), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil for a driver with no laps, got %#v", got)
		}
	})
}

// TestRaceLiveUseCase_FilterPassThrough proves the remaining GetX methods are thin, unmodified
// pass-throughs to the repository (normal case), including the query filters being forwarded
// intact rather than re-interpreted by the usecase.
func TestRaceLiveUseCase_FilterPassThrough(t *testing.T) {
	repo := &fakeRaceLiveRepository{
		stints:   []domain.RaceStint{{DriverNumber: 1, StintNumber: 1}},
		pitStops: []domain.RacePitStop{{DriverNumber: 1, LapNumber: 12}},
		weather:  []domain.RaceWeatherSample{{}},
		radio:    []domain.RaceRadioMessage{{DriverNumber: 1}},
	}
	uc := newRaceLiveUseCase(repo, &fakeChampionshipClientForLive{}, &fakeRaceControlBroadcaster{})

	if stints, err := uc.GetStints(context.Background(), "s1", nil); err != nil || len(stints) != 1 {
		t.Fatalf("GetStints: expected 1 item, nil err, got %v %v", stints, err)
	}
	if pit, err := uc.GetPitStops(context.Background(), "s1", nil); err != nil || len(pit) != 1 {
		t.Fatalf("GetPitStops: expected 1 item, nil err, got %v %v", pit, err)
	}
	if weather, err := uc.GetWeather(context.Background(), "s1"); err != nil || len(weather) != 1 {
		t.Fatalf("GetWeather: expected 1 item, nil err, got %v %v", weather, err)
	}
	if _, err := uc.GetRadio(context.Background(), "s1", intPtr(7)); err != nil {
		t.Fatalf("GetRadio: unexpected error %v", err)
	}
	if repo.lastListRadioDriver == nil || *repo.lastListRadioDriver != 7 {
		t.Fatalf("expected driverNumber=7 to be forwarded to the repository, got %v", repo.lastListRadioDriver)
	}
}

// TestRaceLiveUseCase_WaitForRaceControl proves the long-poll entrypoint forwards sessionID and
// timeout to the broadcaster unmodified and returns whatever it yields (the broadcaster's own
// wake/timeout/cancellation/isolation semantics are covered directly in
// race_control_broadcaster_test.go).
func TestRaceLiveUseCase_WaitForRaceControl(t *testing.T) {
	broadcaster := &fakeRaceControlBroadcaster{waitReturn: []domain.RaceControlEvent{{Message: "flag"}}}
	uc := newRaceLiveUseCase(&fakeRaceLiveRepository{}, &fakeChampionshipClientForLive{}, broadcaster)

	got, err := uc.WaitForRaceControl(context.Background(), "s1", 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Message != "flag" {
		t.Fatalf("expected the broadcaster's return value to be forwarded, got %+v", got)
	}
	if broadcaster.lastWaitSessionID != "s1" || broadcaster.lastWaitTimeout != 30*time.Second {
		t.Fatalf("expected sessionID/timeout to be forwarded unmodified, got sessionID=%q timeout=%v", broadcaster.lastWaitSessionID, broadcaster.lastWaitTimeout)
	}
}
