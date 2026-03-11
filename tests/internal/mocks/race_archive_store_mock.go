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
	GetChampionshipEventsFn     func(context.Context, string) (domain.ChampionshipSummary, []domain.EventSummary, bool, error)
	GetEventFn                  func(context.Context, string) (domain.EventSummary, bool, error)
	ListEventSessionsFn         func(context.Context, string) ([]domain.SessionSummary, bool, error)
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

// GetChampionshipEvents returns the event catalog mock response for one championship.
func (m *RaceArchiveStoreMock) GetChampionshipEvents(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.EventSummary, bool, error) {
	if m.GetChampionshipEventsFn != nil {
		return m.GetChampionshipEventsFn(ctx, code)
	}
	return domain.ChampionshipSummary{}, nil, false, nil
}

// GetEvent returns the event mock response for one event identifier.
func (m *RaceArchiveStoreMock) GetEvent(ctx context.Context, eventID string) (domain.EventSummary, bool, error) {
	if m.GetEventFn != nil {
		return m.GetEventFn(ctx, eventID)
	}
	return domain.EventSummary{}, false, nil
}

// ListEventSessions returns the session catalog mock response for one event.
func (m *RaceArchiveStoreMock) ListEventSessions(ctx context.Context, eventID string) ([]domain.SessionSummary, bool, error) {
	if m.ListEventSessionsFn != nil {
		return m.ListEventSessionsFn(ctx, eventID)
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
