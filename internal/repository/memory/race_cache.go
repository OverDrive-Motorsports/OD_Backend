/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_cache.go - In-memory thread-safe implementation of the race cache repository.
##
*/

package memory

import (
	"context"
	"sync"
	"time"

	"overdrive/internal/domain"
)

type RaceCache struct {
	mu       sync.RWMutex
	archive  *domain.RaceArchive
	cachedAt time.Time
}

// NewRaceCache initializes an empty in-memory race cache repository.
func NewRaceCache() *RaceCache {
	return &RaceCache{}
}

// Store saves a race archive snapshot and its cache timestamp in memory.
func (c *RaceCache) Store(_ context.Context, archive domain.RaceArchive, cachedAt time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	clone := archive
	c.archive = &clone
	c.cachedAt = cachedAt
	return nil
}

// Get returns the current cached race archive snapshot, if any.
func (c *RaceCache) Get(_ context.Context) (domain.RaceArchive, time.Time, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.archive == nil {
		return domain.RaceArchive{}, time.Time{}, false, nil
	}
	return *c.archive, c.cachedAt, true, nil
}
