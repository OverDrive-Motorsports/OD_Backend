/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_store_mock.go - Repository mock used by unit tests for service and API layers.
##
*/

package mocks

import (
	"context"
	"time"

	"overdrive/internal/domain"
)

// RaceArchiveStoreMock implements the race archive repository contract with function fields.
type RaceArchiveStoreMock struct {
	StoreFn                     func(context.Context, domain.RaceArchive) (time.Time, error)
	GetLatestFn                 func(context.Context) (domain.RaceArchive, time.Time, bool, error)
	GetLatestMergedFn           func(context.Context) (domain.RaceArchive, time.Time, bool, error)
	GetSessionMergedFn          func(context.Context, string) (domain.RaceArchive, time.Time, bool, error)
	GetSessionDriverBroadcastFn func(context.Context, string, int) (string, bool, error)
	ListChampionshipsFn         func(context.Context) ([]domain.ChampionshipSummary, error)
	GetChampionshipRacesFn      func(context.Context, string) (domain.ChampionshipSummary, []domain.RaceSummary, bool, error)
	GetRaceFn                   func(context.Context, string) (domain.RaceSummary, bool, error)
	ListRaceSessionsFn          func(context.Context, string) ([]domain.SessionSummary, bool, error)
	GetSessionFn                func(context.Context, string) (domain.SessionSummary, bool, error)
}

// Store records a race archive persistence request.
func (m *RaceArchiveStoreMock) Store(ctx context.Context, archive domain.RaceArchive) (time.Time, error) {
	if m.StoreFn != nil {
		return m.StoreFn(ctx, archive)
	}
	return time.Time{}, nil
}

// GetLatest returns the latest stored archive mock response.
func (m *RaceArchiveStoreMock) GetLatest(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	if m.GetLatestFn != nil {
		return m.GetLatestFn(ctx)
	}
	return domain.RaceArchive{}, time.Time{}, false, nil
}

// GetLatestMerged returns the latest merged archive mock response.
func (m *RaceArchiveStoreMock) GetLatestMerged(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	if m.GetLatestMergedFn != nil {
		return m.GetLatestMergedFn(ctx)
	}
	return domain.RaceArchive{}, time.Time{}, false, nil
}

// GetSessionMerged returns the merged archive mock response for one explicit session.
func (m *RaceArchiveStoreMock) GetSessionMerged(ctx context.Context, sessionID string) (domain.RaceArchive, time.Time, bool, error) {
	if m.GetSessionMergedFn != nil {
		return m.GetSessionMergedFn(ctx, sessionID)
	}
	return domain.RaceArchive{}, time.Time{}, false, nil
}

// GetSessionDriverBroadcast returns the mock broadcast URL for one driver in one session.
func (m *RaceArchiveStoreMock) GetSessionDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (string, bool, error) {
	if m.GetSessionDriverBroadcastFn != nil {
		return m.GetSessionDriverBroadcastFn(ctx, sessionID, driverNumber)
	}
	return "", false, nil
}

// ListChampionships returns the championship catalog mock response.
func (m *RaceArchiveStoreMock) ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error) {
	if m.ListChampionshipsFn != nil {
		return m.ListChampionshipsFn(ctx)
	}
	return nil, nil
}

// GetChampionshipRaces returns the race catalog mock response for one championship.
func (m *RaceArchiveStoreMock) GetChampionshipRaces(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.RaceSummary, bool, error) {
	if m.GetChampionshipRacesFn != nil {
		return m.GetChampionshipRacesFn(ctx, code)
	}
	return domain.ChampionshipSummary{}, nil, false, nil
}

// GetRace returns the race mock response for one race identifier.
func (m *RaceArchiveStoreMock) GetRace(ctx context.Context, raceID string) (domain.RaceSummary, bool, error) {
	if m.GetRaceFn != nil {
		return m.GetRaceFn(ctx, raceID)
	}
	return domain.RaceSummary{}, false, nil
}

// ListRaceSessions returns the session catalog mock response for one race.
func (m *RaceArchiveStoreMock) ListRaceSessions(ctx context.Context, raceID string) ([]domain.SessionSummary, bool, error) {
	if m.ListRaceSessionsFn != nil {
		return m.ListRaceSessionsFn(ctx, raceID)
	}
	return nil, false, nil
}

// GetSession returns the mock response for one session identifier.
func (m *RaceArchiveStoreMock) GetSession(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error) {
	if m.GetSessionFn != nil {
		return m.GetSessionFn(ctx, sessionID)
	}
	return domain.SessionSummary{}, false, nil
}
