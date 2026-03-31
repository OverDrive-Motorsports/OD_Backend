/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_service.go - Application service orchestrating race fetch and persistent archive storage.
##
*/

package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"overdrive/internal/domain"
	"overdrive/internal/repository"
	"overdrive/internal/usecase"
)

type RaceService struct {
	builder *usecase.RaceBuilder
	store   repository.RaceArchiveStoreRepository
	fetchMu sync.Mutex
}

// NewRaceService creates the race application service with build and persistent storage dependencies.
func NewRaceService(builder *usecase.RaceBuilder, store repository.RaceArchiveStoreRepository) *RaceService {
	return &RaceService{
		builder: builder,
		store:   store,
	}
}

// FetchAndStore builds a race archive from provider data and persists it in storage.
func (s *RaceService) FetchAndStore(ctx context.Context, in usecase.RaceBuildInput) (domain.RaceArchive, error) {
	// Prevent burst duplicate provider calls when multiple clients trigger getrace together.
	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()

	archive, err := s.builder.Build(ctx, in)
	if err != nil {
		return domain.RaceArchive{}, err
	}

	storedAt, err := s.store.Store(ctx, archive)
	if err != nil {
		return domain.RaceArchive{}, fmt.Errorf("archive store failed: %w", err)
	}

	if archive.Metadata.GeneratedAt.IsZero() {
		archive.Metadata.GeneratedAt = storedAt
	}

	return archive, nil
}

// GetLatestStored returns the latest stored race archive snapshot when available.
func (s *RaceService) GetLatestStored(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	archive, storedAt, found, err := s.store.GetLatest(ctx)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("archive read failed: %w", err)
	}
	if !found {
		return domain.RaceArchive{}, time.Time{}, false, nil
	}
	return archive, storedAt, true, nil
}

// GetLatestMergedStored returns the merged latest stored race snapshot for the active session.
func (s *RaceService) GetLatestMergedStored(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	archive, storedAt, found, err := s.store.GetLatestMerged(ctx)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("archive read failed: %w", err)
	}
	if !found {
		return domain.RaceArchive{}, time.Time{}, false, nil
	}
	return archive, storedAt, true, nil
}

// GetSessionMergedStored returns the merged stored race snapshot for one explicit session.
func (s *RaceService) GetSessionMergedStored(ctx context.Context, sessionID string) (domain.RaceArchive, time.Time, bool, error) {
	archive, storedAt, found, err := s.store.GetSessionMerged(ctx, sessionID)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("session archive read failed: %w", err)
	}
	if !found {
		return domain.RaceArchive{}, time.Time{}, false, nil
	}
	return archive, storedAt, true, nil
}

// GetSessionMetadata returns one light metadata envelope for one session when supported by storage.
func (s *RaceService) GetSessionMetadata(ctx context.Context, sessionID string) (domain.SessionMetadataWindow, bool, error) {
	window, direct, err := s.store.GetSessionMetadata(ctx, sessionID)
	if err != nil {
		return domain.SessionMetadataWindow{}, false, fmt.Errorf("session metadata lookup failed: %w", err)
	}
	return window, direct, nil
}

// GetSessionDatasetCatalog returns the light dataset catalog for one session when supported by storage.
func (s *RaceService) GetSessionDatasetCatalog(ctx context.Context, sessionID string) (domain.SessionDatasetCatalogWindow, bool, error) {
	window, direct, err := s.store.GetSessionDatasetCatalog(ctx, sessionID)
	if err != nil {
		return domain.SessionDatasetCatalogWindow{}, false, fmt.Errorf("session dataset catalog lookup failed: %w", err)
	}
	return window, direct, nil
}

// GetSessionDriverBroadcast returns the broadcast URL for one driver in one session.
func (s *RaceService) GetSessionDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (string, bool, error) {
	url, found, err := s.store.GetSessionDriverBroadcast(ctx, sessionID, driverNumber)
	if err != nil {
		return "", false, fmt.Errorf("session driver broadcast lookup failed: %w", err)
	}
	return url, found, nil
}

// GetSessionDataset returns one session-scoped dataset directly from normalized storage when supported.
func (s *RaceService) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetWindow, bool, error) {
	window, direct, err := s.store.GetSessionDataset(ctx, sessionID, dataset)
	if err != nil {
		return domain.SessionDatasetWindow{}, false, fmt.Errorf("session dataset lookup failed: %w", err)
	}
	return window, direct, nil
}

// GetSessionDriverDataset returns one driver-scoped dataset directly from normalized storage when supported.
func (s *RaceService) GetSessionDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) (domain.DriverDatasetWindow, bool, error) {
	window, direct, err := s.store.GetSessionDriverDataset(ctx, sessionID, driverNumber, dataset)
	if err != nil {
		return domain.DriverDatasetWindow{}, false, fmt.Errorf("session driver dataset lookup failed: %w", err)
	}
	return window, direct, nil
}

// GetSessionDriverLapLocation returns all stored location samples for one driver during one lap in one session.
func (s *RaceService) GetSessionDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (domain.DriverLapLocationWindow, bool, error) {
	window, found, err := s.store.GetSessionDriverLapLocation(ctx, sessionID, driverNumber, lapNumber)
	if err != nil {
		return domain.DriverLapLocationWindow{}, false, fmt.Errorf("session driver lap location lookup failed: %w", err)
	}
	return window, found, nil
}

// GetSessionRaceStandings returns one race standings snapshot directly from normalized storage when supported.
func (s *RaceService) GetSessionRaceStandings(ctx context.Context, sessionID string, at *time.Time) (domain.RaceStandingsWindow, bool, error) {
	window, direct, err := s.store.GetSessionRaceStandings(ctx, sessionID, at)
	if err != nil {
		return domain.RaceStandingsWindow{}, false, fmt.Errorf("session race standings lookup failed: %w", err)
	}
	return window, direct, nil
}

// ListChampionships returns all stored championships.
func (s *RaceService) ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error) {
	items, err := s.store.ListChampionships(ctx)
	if err != nil {
		return nil, fmt.Errorf("championship list failed: %w", err)
	}
	return items, nil
}

// GetChampionshipEvents returns one championship and its stored events.
func (s *RaceService) GetChampionshipEvents(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.EventSummary, bool, error) {
	championship, events, found, err := s.store.GetChampionshipEvents(ctx, code)
	if err != nil {
		return domain.ChampionshipSummary{}, nil, false, fmt.Errorf("championship events failed: %w", err)
	}
	return championship, events, found, nil
}

// GetEvent returns one stored event by identifier.
func (s *RaceService) GetEvent(ctx context.Context, eventID string) (domain.EventSummary, bool, error) {
	event, found, err := s.store.GetEvent(ctx, eventID)
	if err != nil {
		return domain.EventSummary{}, false, fmt.Errorf("event lookup failed: %w", err)
	}
	return event, found, nil
}

// ListEventSessions returns the stored sessions belonging to one event.
func (s *RaceService) ListEventSessions(ctx context.Context, eventID string) ([]domain.SessionSummary, bool, error) {
	items, found, err := s.store.ListEventSessions(ctx, eventID)
	if err != nil {
		return nil, false, fmt.Errorf("event session list failed: %w", err)
	}
	return items, found, nil
}

// GetSession returns one stored session by identifier.
func (s *RaceService) GetSession(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error) {
	session, found, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return domain.SessionSummary{}, false, fmt.Errorf("session lookup failed: %w", err)
	}
	return session, found, nil
}
