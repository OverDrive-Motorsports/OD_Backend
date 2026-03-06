/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_service.go - Application service orchestrating race fetch and cache operations.
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
	cache   repository.RaceCacheRepository
	fetchMu sync.Mutex
}

// NewRaceService creates the race application service with build and cache dependencies.
func NewRaceService(builder *usecase.RaceBuilder, cache repository.RaceCacheRepository) *RaceService {
	return &RaceService{
		builder: builder,
		cache:   cache,
	}
}

// FetchAndCache builds a race archive from provider data and persists it in cache.
func (s *RaceService) FetchAndCache(ctx context.Context, in usecase.RaceBuildInput) (domain.RaceArchive, error) {
	// Prevent burst duplicate provider calls when multiple clients trigger getrace together.
	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()

	archive, err := s.builder.Build(ctx, in)
	if err != nil {
		return domain.RaceArchive{}, err
	}

	cachedAt := time.Now().UTC()
	if err := s.cache.Store(ctx, archive, cachedAt); err != nil {
		return domain.RaceArchive{}, fmt.Errorf("cache store failed: %w", err)
	}

	return archive, nil
}

// GetCached returns the latest cached race archive snapshot when available.
func (s *RaceService) GetCached(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	archive, cachedAt, found, err := s.cache.Get(ctx)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("cache get failed: %w", err)
	}
	if !found {
		return domain.RaceArchive{}, time.Time{}, false, nil
	}
	return archive, cachedAt, true, nil
}
