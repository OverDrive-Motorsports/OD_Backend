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
	StoreFn                       func(context.Context, domain.RaceArchive) (time.Time, error)
	GetLatestFn                   func(context.Context) (domain.RaceArchive, time.Time, bool, error)
	GetLatestMergedFn             func(context.Context) (domain.RaceArchive, time.Time, bool, error)
	GetSessionMergedFn            func(context.Context, string) (domain.RaceArchive, time.Time, bool, error)
	GetSessionMetadataFn          func(context.Context, string) (domain.SessionMetadataWindow, bool, error)
	GetSessionDatasetCatalogFn    func(context.Context, string) (domain.SessionDatasetCatalogWindow, bool, error)
	GetSessionDriverBroadcastFn   func(context.Context, string, int) (string, bool, error)
	GetSessionDatasetFn           func(context.Context, string, string) (domain.SessionDatasetWindow, bool, error)
	GetSessionDriverDatasetFn     func(context.Context, string, int, string) (domain.DriverDatasetWindow, bool, error)
	GetSessionDriverLapLocationFn func(context.Context, string, int, int) (domain.DriverLapLocationWindow, bool, error)
	GetSessionRaceStandingsFn     func(context.Context, string, *time.Time) (domain.RaceStandingsWindow, bool, error)
	ListChampionshipsFn           func(context.Context) ([]domain.ChampionshipSummary, error)
	GetChampionshipEventsFn       func(context.Context, string) (domain.ChampionshipSummary, []domain.EventSummary, bool, error)
	GetEventFn                    func(context.Context, string) (domain.EventSummary, bool, error)
	ListEventSessionsFn           func(context.Context, string) ([]domain.SessionSummary, bool, error)
	GetSessionFn                  func(context.Context, string) (domain.SessionSummary, bool, error)
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

// GetSessionMetadata returns the mock metadata envelope for one session.
func (m *RaceArchiveStoreMock) GetSessionMetadata(ctx context.Context, sessionID string) (domain.SessionMetadataWindow, bool, error) {
	if m.GetSessionMetadataFn != nil {
		return m.GetSessionMetadataFn(ctx, sessionID)
	}
	return domain.SessionMetadataWindow{}, false, nil
}

// GetSessionDatasetCatalog returns the mock dataset catalog for one session.
func (m *RaceArchiveStoreMock) GetSessionDatasetCatalog(ctx context.Context, sessionID string) (domain.SessionDatasetCatalogWindow, bool, error) {
	if m.GetSessionDatasetCatalogFn != nil {
		return m.GetSessionDatasetCatalogFn(ctx, sessionID)
	}
	return domain.SessionDatasetCatalogWindow{}, false, nil
}

// GetSessionDriverBroadcast returns the mock broadcast URL for one driver in one session.
func (m *RaceArchiveStoreMock) GetSessionDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (string, bool, error) {
	if m.GetSessionDriverBroadcastFn != nil {
		return m.GetSessionDriverBroadcastFn(ctx, sessionID, driverNumber)
	}
	return "", false, nil
}

// GetSessionDataset returns the mock direct dataset window for one session.
func (m *RaceArchiveStoreMock) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetWindow, bool, error) {
	if m.GetSessionDatasetFn != nil {
		return m.GetSessionDatasetFn(ctx, sessionID, dataset)
	}
	return domain.SessionDatasetWindow{}, false, nil
}

// GetSessionDriverDataset returns the mock direct dataset window for one driver in one session.
func (m *RaceArchiveStoreMock) GetSessionDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) (domain.DriverDatasetWindow, bool, error) {
	if m.GetSessionDriverDatasetFn != nil {
		return m.GetSessionDriverDatasetFn(ctx, sessionID, driverNumber, dataset)
	}
	return domain.DriverDatasetWindow{}, false, nil
}

// GetSessionDriverLapLocation returns the mock lap-scoped location window for one driver in one session.
func (m *RaceArchiveStoreMock) GetSessionDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (domain.DriverLapLocationWindow, bool, error) {
	if m.GetSessionDriverLapLocationFn != nil {
		return m.GetSessionDriverLapLocationFn(ctx, sessionID, driverNumber, lapNumber)
	}
	return domain.DriverLapLocationWindow{}, false, nil
}

// GetSessionRaceStandings returns the mock direct standings snapshot for one session.
func (m *RaceArchiveStoreMock) GetSessionRaceStandings(ctx context.Context, sessionID string, at *time.Time) (domain.RaceStandingsWindow, bool, error) {
	if m.GetSessionRaceStandingsFn != nil {
		return m.GetSessionRaceStandingsFn(ctx, sessionID, at)
	}
	return domain.RaceStandingsWindow{}, false, nil
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
