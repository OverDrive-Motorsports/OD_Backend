/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## telemetry_test.go - Package usecases source file for services/race-data-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"
	"errors"
	"testing"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// fakeTelemetryRepository is an in-memory ports.TelemetryRepository used to exercise
// TelemetryUseCase without a DB.
type fakeTelemetryRepository struct {
	speed      []domain.TelemetrySpeed
	engine     []domain.TelemetryEngine
	location   []domain.TelemetryLocation
	intervals  *domain.TelemetryIntervals
	err        error
	lastLapArg *int
}

func (f *fakeTelemetryRepository) GetSpeed(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetrySpeed, error) {
	f.lastLapArg = lapNumber
	return f.speed, f.err
}

func (f *fakeTelemetryRepository) GetEngine(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryEngine, error) {
	f.lastLapArg = lapNumber
	return f.engine, f.err
}

func (f *fakeTelemetryRepository) ListLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryLocation, error) {
	f.lastLapArg = lapNumber
	return f.location, f.err
}

func (f *fakeTelemetryRepository) GetIntervals(ctx context.Context, sessionID string, driverNumber int) (*domain.TelemetryIntervals, error) {
	return f.intervals, f.err
}

// fakeChampionshipClientForTelemetry only implements GetSession, the only method
// TelemetryUseCase (via SessionExists) actually calls.
type fakeChampionshipClientForTelemetry struct {
	ports.ChampionshipClient
	session *ports.ChampionshipSessionRef
	err     error
}

func (f *fakeChampionshipClientForTelemetry) GetSession(ctx context.Context, sessionID string) (*ports.ChampionshipSessionRef, error) {
	return f.session, f.err
}

// TestTelemetryUseCase_SessionExists mirrors RaceLiveUseCase's SessionExists coverage - the same
// "unknown sessionId -> 404" contract backs every /telemetry/* endpoint.
func TestTelemetryUseCase_SessionExists(t *testing.T) {
	t.Run("known session", func(t *testing.T) {
		uc := NewTelemetryUseCase(&fakeTelemetryRepository{}, &fakeChampionshipClientForTelemetry{session: &ports.ChampionshipSessionRef{ID: "s1"}})
		exists, err := uc.SessionExists(context.Background(), "s1")
		if err != nil || !exists {
			t.Fatalf("expected exists=true, err=nil, got exists=%v err=%v", exists, err)
		}
	})

	t.Run("unknown session", func(t *testing.T) {
		uc := NewTelemetryUseCase(&fakeTelemetryRepository{}, &fakeChampionshipClientForTelemetry{session: nil})
		exists, err := uc.SessionExists(context.Background(), "does-not-exist")
		if err != nil || exists {
			t.Fatalf("expected exists=false, err=nil, got exists=%v err=%v", exists, err)
		}
	})
}

// TestTelemetryUseCase_GetSpeed_NormalCase proves the normal pass-through case: the repository's
func TestTelemetryUseCase_GetSpeed_NormalCase(t *testing.T) {
	repo := &fakeTelemetryRepository{speed: []domain.TelemetrySpeed{{Speed: 300, Gear: 8}, {Speed: 310, Gear: 8}}}
	uc := NewTelemetryUseCase(repo, &fakeChampionshipClientForTelemetry{})

	got, err := uc.GetSpeed(context.Background(), "s1", 44, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[1].Speed != 310 {
		t.Fatalf("expected the repository's samples to be returned unmodified, got %+v", got)
	}
}

// TestTelemetryUseCase_GetEngine_LapFilterForwarded proves the lapNumber query filter is
// forwarded to the repository unmodified rather than being reinterpreted by the usecase.
func TestTelemetryUseCase_GetEngine_LapFilterForwarded(t *testing.T) {
	repo := &fakeTelemetryRepository{engine: []domain.TelemetryEngine{{Rpm: 11000, DrsActive: true}}}
	uc := NewTelemetryUseCase(repo, &fakeChampionshipClientForTelemetry{})

	lap := 3
	got, err := uc.GetEngine(context.Background(), "s1", 44, &lap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || !got[0].DrsActive {
		t.Fatalf("expected the repository's samples to be returned unmodified, got %+v", got)
	}
	if repo.lastLapArg == nil || *repo.lastLapArg != 3 {
		t.Fatalf("expected lapNumber=3 to be forwarded to the repository, got %v", repo.lastLapArg)
	}
}

// TestTelemetryUseCase_GetLocation_EmptyForUnknownDriver proves an empty result set (unknown
// driver/session) is returned as an empty slice rather than swallowed into a nil-with-no-error
// ambiguity - the repository is the source of truth for that shape, the usecase must not alter it.
func TestTelemetryUseCase_GetLocation_EmptyForUnknownDriver(t *testing.T) {
	repo := &fakeTelemetryRepository{location: []domain.TelemetryLocation{}}
	uc := NewTelemetryUseCase(repo, &fakeChampionshipClientForTelemetry{})

	got, err := uc.GetLocation(context.Background(), "s1", 999, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected an empty slice, got %+v", got)
	}
}

// TestTelemetryUseCase_GetIntervals_NilWhenNoSample proves GetIntervals surfaces a nil pointer
// (translated by the controller into a 404) instead of returning a zero-value struct when the
// repository has no sample for the driver.
func TestTelemetryUseCase_GetIntervals_NilWhenNoSample(t *testing.T) {
	uc := NewTelemetryUseCase(&fakeTelemetryRepository{intervals: nil}, &fakeChampionshipClientForTelemetry{})

	got, err := uc.GetIntervals(context.Background(), "s1", 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// TestTelemetryUseCase_RepositoryErrorPropagates proves a repository failure is neither
// swallowed nor wrapped/altered - it must reach the controller untouched so it can be classified
// into the right apierror status.
func TestTelemetryUseCase_RepositoryErrorPropagates(t *testing.T) {
	boom := errors.New("db unavailable")
	uc := NewTelemetryUseCase(&fakeTelemetryRepository{err: boom}, &fakeChampionshipClientForTelemetry{})

	if _, err := uc.GetSpeed(context.Background(), "s1", 44, nil); !errors.Is(err, boom) {
		t.Fatalf("expected the repository error to propagate, got %v", err)
	}
}
