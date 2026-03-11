/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_store.go - Prisma-backed repository entrypoints for storing and reading race archives.
##
*/

package prisma

import (
	"context"
	"fmt"
	"time"

	"overdrive/internal/domain"
	"overdrive/resources/db"
)

const (
	providerCodeOpenF1     = "openf1"
	providerNameOpenF1     = "OpenF1"
	championshipCodeF1     = "f1"
	championshipNameF1     = "Formula 1"
	championshipCategoryF1 = "single-seater"
	rawDatasetChunkSize    = 5000
)

type RaceArchiveStore struct {
	client *db.PrismaClient
}

type metadataEnvelope struct {
	Metadata    domain.Metadata  `json:"metadata"`
	Meeting     map[string]any   `json:"meeting"`
	AllSessions []map[string]any `json:"all_sessions"`
	RaceSession map[string]any   `json:"race_session"`
}

// NewRaceArchiveStore builds a Prisma-backed race archive repository.
func NewRaceArchiveStore(client *db.PrismaClient) *RaceArchiveStore {
	return &RaceArchiveStore{client: client}
}

// Store persists one fetched race archive in PostgreSQL.
func (s *RaceArchiveStore) Store(ctx context.Context, archive domain.RaceArchive) (time.Time, error) {
	if s.client == nil {
		return time.Time{}, fmt.Errorf("prisma client is nil")
	}

	provider, err := s.ensureProvider(ctx)
	if err != nil {
		return time.Time{}, err
	}

	event, err := s.ensureEvent(ctx, provider.ID, archive)
	if err != nil {
		return time.Time{}, err
	}

	championship, err := s.ensureChampionship(ctx, provider.ID)
	if err != nil {
		return time.Time{}, err
	}

	race, err := s.ensureRace(ctx, championship.ID, event.ID, archive)
	if err != nil {
		return time.Time{}, err
	}

	session, err := s.ensureSession(ctx, event.ID, race.ID, archive)
	if err != nil {
		return time.Time{}, err
	}

	if err := s.syncParticipants(ctx, provider.ID, archive.Datasets["drivers"]); err != nil {
		return time.Time{}, err
	}

	metadataJSON, err := marshalPrismaJSON(metadataEnvelope{
		Metadata:    archive.Metadata,
		Meeting:     archive.Meeting,
		AllSessions: archive.AllSessions,
		RaceSession: archive.RaceSession,
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("marshal metadata envelope: %w", err)
	}

	countsJSON, err := marshalPrismaJSON(archive.Counts)
	if err != nil {
		return time.Time{}, fmt.Errorf("marshal counts: %w", err)
	}

	generatedAt := archive.Metadata.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	archiveID, err := newUUID()
	if err != nil {
		return time.Time{}, fmt.Errorf("generate archive id: %w", err)
	}

	archiveParams := []db.RaceArchiveSetParam{
		db.RaceArchive.ID.Set(archiveID),
		db.RaceArchive.Mode.Set(resolveArchiveMode(archive.Metadata.DriverNumber)),
		db.RaceArchive.GeneratedAt.Set(generatedAt),
		db.RaceArchive.SourceMeetingKey.SetIfPresent(optionalInt(archive.Metadata.MeetingKey)),
		db.RaceArchive.SourceSessionKey.SetIfPresent(optionalInt(archive.Metadata.RaceSessKey)),
		db.RaceArchive.SourceDriverNumber.SetIfPresent(optionalInt(archive.Metadata.DriverNumber)),
	}

	if len(archive.FetchErrors) > 0 {
		fetchErrorsJSON, err := marshalPrismaJSON(archive.FetchErrors)
		if err != nil {
			return time.Time{}, fmt.Errorf("marshal fetch_errors: %w", err)
		}
		archiveParams = append(archiveParams, db.RaceArchive.FetchErrors.Set(fetchErrorsJSON))
	}

	txQueries := []db.PrismaTransaction{
		s.client.RaceArchive.CreateOne(
			db.RaceArchive.Metadata.Set(metadataJSON),
			db.RaceArchive.Counts.Set(countsJSON),
			db.RaceArchive.Session.Link(db.Session.ID.Equals(session.ID)),
			archiveParams...,
		).Tx(),
	}

	chunkQueries, err := s.buildDatasetChunkQueries(archiveID, archive.Datasets)
	if err != nil {
		return time.Time{}, err
	}
	txQueries = append(txQueries, chunkQueries...)

	normalizedQueries, err := s.storeNormalizedSessionDataTx(session.ID, archive)
	if err != nil {
		return time.Time{}, err
	}
	txQueries = append(txQueries, normalizedQueries...)

	if err := s.client.Prisma.Transaction(txQueries...).Exec(ctx); err != nil {
		return time.Time{}, fmt.Errorf("commit race archive transaction: %w", err)
	}

	return generatedAt, nil
}

// GetLatest returns the latest persisted race archive from PostgreSQL.
func (s *RaceArchiveStore) GetLatest(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	if s.client == nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("prisma client is nil")
	}

	latest, err := s.client.RaceArchive.FindFirst().
		OrderBy(db.RaceArchive.GeneratedAt.Order(db.SortOrderDesc)).
		Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.RaceArchive{}, time.Time{}, false, nil
		}
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("find latest race archive: %w", err)
	}

	archive, err := s.archiveFromModel(ctx, latest)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, err
	}

	return archive, latest.GeneratedAt, true, nil
}

// GetLatestMerged returns a merged archive view across all imports for the latest stored session.
func (s *RaceArchiveStore) GetLatestMerged(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	if s.client == nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("prisma client is nil")
	}

	latest, err := s.client.RaceArchive.FindFirst().
		OrderBy(db.RaceArchive.GeneratedAt.Order(db.SortOrderDesc)).
		Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.RaceArchive{}, time.Time{}, false, nil
		}
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("find latest race archive: %w", err)
	}

	archives, err := s.client.RaceArchive.FindMany(
		db.RaceArchive.SessionID.Equals(latest.SessionID),
	).
		OrderBy(db.RaceArchive.GeneratedAt.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("find session race archives: %w", err)
	}

	merged, err := s.mergeArchiveModels(ctx, archives)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, err
	}

	return merged, latest.GeneratedAt, true, nil
}
