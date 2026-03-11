/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_store.go - Repository contract for persisting and reading race archives from storage.
##
*/

package repository

import (
	"context"
	"time"

	"overdrive/internal/domain"
)

// RaceArchiveStoreRepository abstracts persistent storage of race archives.
type RaceArchiveStoreRepository interface {
	Store(ctx context.Context, archive domain.RaceArchive) (storedAt time.Time, err error)
	GetLatest(ctx context.Context) (archive domain.RaceArchive, storedAt time.Time, found bool, err error)
	GetLatestMerged(ctx context.Context) (archive domain.RaceArchive, storedAt time.Time, found bool, err error)
	GetSessionMerged(ctx context.Context, sessionID string) (archive domain.RaceArchive, storedAt time.Time, found bool, err error)
	GetSessionDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (url string, found bool, err error)
	ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error)
	GetChampionshipRaces(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.RaceSummary, bool, error)
	GetRace(ctx context.Context, raceID string) (domain.RaceSummary, bool, error)
	ListRaceSessions(ctx context.Context, raceID string) ([]domain.SessionSummary, bool, error)
	GetSession(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error)
}
