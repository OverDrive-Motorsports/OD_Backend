/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## race_cache.go - Repository contract for storing and retrieving race cache snapshots.
 ##
 */

package repository

import (
	"context"
	"time"

	"overdrive/internal/domain"
)

// RaceCacheRepository abstracts the storage of the latest fetched race payload.
// V1 uses in-memory cache; DB/Redis can implement this later without touching handlers.
type RaceCacheRepository interface {
	Store(ctx context.Context, archive domain.RaceArchive, cachedAt time.Time) error
	Get(ctx context.Context) (archive domain.RaceArchive, cachedAt time.Time, found bool, err error)
}
