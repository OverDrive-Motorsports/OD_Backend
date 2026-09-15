/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_replay_stream_test.go - Package usecases source file for services/race-data-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// fakeRaceStreamRepository is an in-memory ports.RaceStreamRepository used to
// exercise RaceReplayUseCase.GetReplay's grouping logic without a DB.
type fakeRaceStreamRepository struct {
	telemetry   []domain.RaceReplayTelemetryFrame
	positions   []domain.RacePosition
	raceControl []domain.RaceControlEvent
	weather     []domain.RaceWeatherSample
	radio       []domain.RaceRadioMessage
	pitStops    []ports.RaceReplayPitStopFrame
	stints      []ports.RaceReplayStintFrame
	laps        []ports.RaceReplayLapFrame
}

func (f *fakeRaceStreamRepository) ListTelemetryFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceReplayTelemetryFrame, error) {
	return f.telemetry, nil
}

func (f *fakeRaceStreamRepository) ListPositionFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePosition, error) {
	return f.positions, nil
}

func (f *fakeRaceStreamRepository) ListRaceControlFrames(ctx context.Context, sessionID string) ([]domain.RaceControlEvent, error) {
	return f.raceControl, nil
}

func (f *fakeRaceStreamRepository) ListWeatherFrames(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error) {
	return f.weather, nil
}

func (f *fakeRaceStreamRepository) ListRadioFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error) {
	return f.radio, nil
}

func (f *fakeRaceStreamRepository) ListPitStopFrames(ctx context.Context, sessionID string, driverNumber *int) ([]ports.RaceReplayPitStopFrame, error) {
	return f.pitStops, nil
}

func (f *fakeRaceStreamRepository) ListStintFrames(ctx context.Context, sessionID string, driverNumber *int) ([]ports.RaceReplayStintFrame, error) {
	return f.stints, nil
}

func (f *fakeRaceStreamRepository) ListLapFrames(ctx context.Context, sessionID string, driverNumber *int) ([]ports.RaceReplayLapFrame, error) {
	return f.laps, nil
}

// fakeChampionshipClient only implements the subset RaceReplayUseCase
// actually calls (GetSession); every other method is unused in this test.
type fakeChampionshipClient struct {
	ports.ChampionshipClient
	session *ports.ChampionshipSessionRef
}

func (f *fakeChampionshipClient) GetSession(ctx context.Context, sessionID string) (*ports.ChampionshipSessionRef, error) {
	return f.session, nil
}

func t0(offsetSeconds int) time.Time {
	base := time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetSeconds) * time.Second)
}

// TestRaceReplayUseCase_SessionExists mirrors RaceLiveUseCase's/TelemetryUseCase's SessionExists
// coverage - the same "unknown sessionId -> 404" contract backs GET /race/replay.
func TestRaceReplayUseCase_SessionExists(t *testing.T) {
	t.Run("known session", func(t *testing.T) {
		uc := NewRaceReplayStreamUseCase(&fakeRaceStreamRepository{}, &fakeChampionshipClient{session: &ports.ChampionshipSessionRef{ID: "s1"}})
		exists, err := uc.SessionExists(context.Background(), "s1")
		if err != nil || !exists {
			t.Fatalf("expected exists=true, err=nil, got exists=%v err=%v", exists, err)
		}
	})

	t.Run("unknown session", func(t *testing.T) {
		uc := NewRaceReplayStreamUseCase(&fakeRaceStreamRepository{}, &fakeChampionshipClient{session: nil})
		exists, err := uc.SessionExists(context.Background(), "missing")
		if err != nil || exists {
			t.Fatalf("expected exists=false, err=nil, got exists=%v err=%v", exists, err)
		}
	})
}

// TestGetReplayGroupsDriverScopedDatasetsByDriverNumber feeds two drivers'
// worth of samples across every driver-scoped dataset and asserts each ends
// up under the right driver-number key, sorted oldest-first, with the
// track-wide datasets (raceControl/weather) left as flat arrays untouched by
// driver grouping.
func TestGetReplayGroupsDriverScopedDatasetsByDriverNumber(t *testing.T) {
	lapDuration := 91.2
	repo := &fakeRaceStreamRepository{
		telemetry: []domain.RaceReplayTelemetryFrame{
			{DriverNumber: 63, Speed: 300, Timestamp: t0(1)},
			{DriverNumber: 63, Speed: 310, Timestamp: t0(5)},
			{DriverNumber: 44, Speed: 290, Timestamp: t0(2)},
		},
		positions: []domain.RacePosition{
			{DriverNumber: 63, Position: 1, Timestamp: t0(2)},
			{DriverNumber: 44, Position: 2, Timestamp: t0(2)},
		},
		raceControl: []domain.RaceControlEvent{{Message: "green flag", Timestamp: t0(0)}},
		weather:     []domain.RaceWeatherSample{{Timestamp: t0(0)}},
		radio: []domain.RaceRadioMessage{
			{DriverNumber: 63, Timestamp: t0(4)},
		},
		pitStops: []ports.RaceReplayPitStopFrame{
			{Payload: domain.RacePitStop{DriverNumber: 63, LapNumber: 18}, Timestamp: t0(3)},
		},
		stints: []ports.RaceReplayStintFrame{
			{Payload: domain.RaceStint{DriverNumber: 44, LapStart: 1}, Timestamp: t0(6)},
		},
		laps: []ports.RaceReplayLapFrame{
			{Payload: domain.RaceReplayLapEvent{DriverNumber: 63, LapNumber: 1, LapDuration: &lapDuration}, Timestamp: t0(7)},
		},
	}
	usecase := NewRaceReplayStreamUseCase(repo, &fakeChampionshipClient{session: &ports.ChampionshipSessionRef{}})

	replay, err := usecase.GetReplay(context.Background(), "openf1:session:9998", nil)
	if err != nil {
		t.Fatalf("GetReplay returned error: %v", err)
	}

	if got := len(replay.Telemetry["63"]); got != 2 {
		t.Fatalf("expected 2 telemetry samples for driver 63, got %d", got)
	}
	if replay.Telemetry["63"][0].Speed != 300 || replay.Telemetry["63"][1].Speed != 310 {
		t.Fatalf("expected driver 63's telemetry sorted oldest-first, got %+v", replay.Telemetry["63"])
	}
	if got := len(replay.Telemetry["44"]); got != 1 {
		t.Fatalf("expected 1 telemetry sample for driver 44, got %d", got)
	}
	if got := len(replay.Position["63"]); got != 1 || got != len(replay.Position["44"]) {
		t.Fatalf("expected 1 position sample for each driver, got 63=%d 44=%d", len(replay.Position["63"]), len(replay.Position["44"]))
	}
	if got := len(replay.Radio["63"]); got != 1 {
		t.Fatalf("expected 1 radio sample for driver 63, got %d", got)
	}
	if _, ok := replay.Radio["44"]; ok {
		t.Fatalf("did not expect a radio entry for driver 44, none was fed")
	}
	if got := len(replay.PitStop["63"]); got != 1 {
		t.Fatalf("expected 1 pit stop for driver 63, got %d", got)
	}
	if got := len(replay.Stint["44"]); got != 1 {
		t.Fatalf("expected 1 stint for driver 44, got %d", got)
	}
	if got := len(replay.Lap["63"]); got != 1 {
		t.Fatalf("expected 1 lap for driver 63, got %d", got)
	}

	if got := len(replay.RaceControl); got != 1 {
		t.Fatalf("expected 1 track-wide race control event, got %d", got)
	}
	if got := len(replay.Weather); got != 1 {
		t.Fatalf("expected 1 track-wide weather sample, got %d", got)
	}
}

// TestGetReplayFiltersToOneDriverWhenDriverNumberGiven mirrors the repository
// contract (it does the actual filtering) but asserts the usecase's grouping
// still produces at most one key per driver-scoped dataset when the
// repository only returns that driver's rows, and leaves track-wide datasets
// unaffected.
func TestGetReplayFiltersToOneDriverWhenDriverNumberGiven(t *testing.T) {
	repo := &fakeRaceStreamRepository{
		telemetry:   []domain.RaceReplayTelemetryFrame{{DriverNumber: 63, Timestamp: t0(1)}},
		positions:   []domain.RacePosition{{DriverNumber: 63, Timestamp: t0(1)}},
		raceControl: []domain.RaceControlEvent{{Timestamp: t0(0)}},
		weather:     []domain.RaceWeatherSample{{Timestamp: t0(0)}},
	}
	usecase := NewRaceReplayStreamUseCase(repo, &fakeChampionshipClient{session: &ports.ChampionshipSessionRef{}})

	driverNumber := 63
	replay, err := usecase.GetReplay(context.Background(), "openf1:session:9998", &driverNumber)
	if err != nil {
		t.Fatalf("GetReplay returned error: %v", err)
	}

	if len(replay.Telemetry) != 1 {
		t.Fatalf("expected exactly one driver key in telemetry, got %d", len(replay.Telemetry))
	}
	if len(replay.Position) != 1 {
		t.Fatalf("expected exactly one driver key in position, got %d", len(replay.Position))
	}
	if len(replay.RaceControl) != 1 || len(replay.Weather) != 1 {
		t.Fatalf("expected track-wide datasets to be unaffected by driverNumber filtering")
	}
}

// TestGetReplayHandlesEmptySources ensures GetReplay doesn't panic when every
// source (including the ones with nullable/derived timestamps like
// stint/lap) returns no rows at all — the repository is responsible for
// filtering out rows it can't place, so the usecase must tolerate empty
// slices for any subset of sources and return empty (non-nil) maps/slices.
func TestGetReplayHandlesEmptySources(t *testing.T) {
	repo := &fakeRaceStreamRepository{}
	usecase := NewRaceReplayStreamUseCase(repo, &fakeChampionshipClient{session: &ports.ChampionshipSessionRef{}})

	replay, err := usecase.GetReplay(context.Background(), "openf1:session:9998", nil)
	if err != nil {
		t.Fatalf("GetReplay returned error: %v", err)
	}

	if len(replay.Telemetry) != 0 || len(replay.Position) != 0 || len(replay.Radio) != 0 ||
		len(replay.PitStop) != 0 || len(replay.Stint) != 0 || len(replay.Lap) != 0 {
		t.Fatalf("expected all driver-scoped maps to be empty, got %+v", replay)
	}
	if len(replay.RaceControl) != 0 || len(replay.Weather) != 0 {
		t.Fatalf("expected track-wide slices to be empty, got %+v", replay)
	}
}
