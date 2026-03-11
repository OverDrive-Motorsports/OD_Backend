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

// GetSessionDriverBroadcast returns the broadcast URL for one driver in one session.
func (s *RaceService) GetSessionDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (string, bool, error) {
	url, found, err := s.store.GetSessionDriverBroadcast(ctx, sessionID, driverNumber)
	if err != nil {
		return "", false, fmt.Errorf("session driver broadcast lookup failed: %w", err)
	}
	return url, found, nil
}

// ListChampionships returns all stored championships.
func (s *RaceService) ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error) {
	items, err := s.store.ListChampionships(ctx)
	if err != nil {
		return nil, fmt.Errorf("championship list failed: %w", err)
	}
	return items, nil
}

// GetChampionshipRaces returns one championship and its stored races.
func (s *RaceService) GetChampionshipRaces(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.RaceSummary, bool, error) {
	championship, races, found, err := s.store.GetChampionshipRaces(ctx, code)
	if err != nil {
		return domain.ChampionshipSummary{}, nil, false, fmt.Errorf("championship races failed: %w", err)
	}
	return championship, races, found, nil
}

// GetRace returns one stored race by identifier.
func (s *RaceService) GetRace(ctx context.Context, raceID string) (domain.RaceSummary, bool, error) {
	race, found, err := s.store.GetRace(ctx, raceID)
	if err != nil {
		return domain.RaceSummary{}, false, fmt.Errorf("race lookup failed: %w", err)
	}
	return race, found, nil
}

// ListRaceSessions returns the stored sessions belonging to one race.
func (s *RaceService) ListRaceSessions(ctx context.Context, raceID string) ([]domain.SessionSummary, bool, error) {
	items, found, err := s.store.ListRaceSessions(ctx, raceID)
	if err != nil {
		return nil, false, fmt.Errorf("session list failed: %w", err)
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
