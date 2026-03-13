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
	GetSessionMetadata(ctx context.Context, sessionID string) (window domain.SessionMetadataWindow, direct bool, err error)
	GetSessionDatasetCatalog(ctx context.Context, sessionID string) (window domain.SessionDatasetCatalogWindow, direct bool, err error)
	GetSessionDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (url string, found bool, err error)
	GetSessionDataset(ctx context.Context, sessionID string, dataset string) (window domain.SessionDatasetWindow, direct bool, err error)
	GetSessionDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) (window domain.DriverDatasetWindow, direct bool, err error)
	GetSessionDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (window domain.DriverLapLocationWindow, found bool, err error)
	GetSessionRaceStandings(ctx context.Context, sessionID string, at *time.Time) (window domain.RaceStandingsWindow, direct bool, err error)
	ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error)
	GetChampionshipEvents(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.EventSummary, bool, error)
	GetEvent(ctx context.Context, eventID string) (domain.EventSummary, bool, error)
	ListEventSessions(ctx context.Context, eventID string) ([]domain.SessionSummary, bool, error)
	GetSession(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error)
}
