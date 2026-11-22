/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_cache.repository.go - Replay dump cache stored in RaceArchive and RaceDatasetChunk.
##
*/

package prismaadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	db "overdrive/services/race-data-service/resources/db"
	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// errArchiveGone reports that an archive was deleted (invalidated by an ingestion) while it was
// being recorded.
var errArchiveGone = errors.New("replay archive no longer exists")

// An archive is servable once its metadata says complete. Chunk ids and rows are read as text:
// the raw query result does not decode BigInt and large jsonb values into Go types reliably.
const (
	findCompleteArchiveQuery = `SELECT "id" FROM "RaceArchive" WHERE "sessionId" = $1 AND "mode" = 'full' AND ("metadata"->>'complete') = 'true' ORDER BY "generatedAt" DESC LIMIT 1`
	listArchiveChunksQuery   = `SELECT "id"::text AS "id", "dataset", "driverNumber" FROM "RaceDatasetChunk" WHERE "archiveId" = $1 ORDER BY "chunkIndex"`
	chunkRowsQuery           = `SELECT "rows"::text AS "rows" FROM "RaceDatasetChunk" WHERE "id" = $1::bigint`
	lockSessionArchivesQuery = `SELECT pg_advisory_xact_lock(hashtextextended('replay-archive:' || $1::text, 0))`
	completeArchiveQuery     = `UPDATE "RaceArchive" SET "metadata" = '{"complete":true}'::jsonb, "generatedAt" = now() WHERE "id" = $1 AND NOT EXISTS (SELECT 1 FROM "RaceArchive" other WHERE other."sessionId" = $2 AND other."id" <> $1 AND (other."metadata"->>'complete') = 'true')`
	deleteStaleRecordings    = `DELETE FROM "RaceArchive" WHERE "sessionId" = $1 AND "id" <> $2 AND ("metadata"->>'complete') IS DISTINCT FROM 'true' AND "generatedAt" < now() - interval '1 hour'`
	deleteArchiveQuery       = `DELETE FROM "RaceArchive" WHERE "id" = $1`
	deleteSessionArchives    = `DELETE FROM "RaceArchive" WHERE "sessionId" = $1`
)

// ReplayCacheRepository implements ports.ReplayCache. Deleting an archive deletes its chunks
// (ON DELETE CASCADE).
type ReplayCacheRepository struct {
	client *db.PrismaClient
}

// NewReplayCacheRepository builds the replay cache on the race-data database.
func NewReplayCacheRepository(client *db.PrismaClient) *ReplayCacheRepository {
	return &ReplayCacheRepository{client: client}
}

// FindComplete returns the newest complete archive of the session.
func (r *ReplayCacheRepository) FindComplete(ctx context.Context, sessionID string) (string, bool, error) {
	var rows []struct {
		ID string `json:"id"`
	}
	if err := r.client.Prisma.QueryRaw(findCompleteArchiveQuery, sessionID).Exec(ctx, &rows); err != nil {
		return "", false, fmt.Errorf("find replay archive: %w", err)
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return rows[0].ID, true, nil
}

// ListChunks returns the archive's parts in write order, without their rows.
func (r *ReplayCacheRepository) ListChunks(ctx context.Context, archiveID string) ([]ports.ReplayChunkRef, error) {
	var rows []struct {
		ID           string `json:"id"`
		Dataset      string `json:"dataset"`
		DriverNumber *int   `json:"driverNumber"`
	}
	if err := r.client.Prisma.QueryRaw(listArchiveChunksQuery, archiveID).Exec(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list replay chunks: %w", err)
	}
	refs := make([]ports.ReplayChunkRef, 0, len(rows))
	for _, row := range rows {
		id, err := strconv.ParseInt(row.ID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("replay chunk id %q: %w", row.ID, err)
		}
		refs = append(refs, ports.ReplayChunkRef{ID: id, Dataset: row.Dataset, DriverNumber: row.DriverNumber})
	}
	return refs, nil
}

// ChunkRows returns the JSON array of one part, compacted (jsonb prints spaces after ':' and ',').
func (r *ReplayCacheRepository) ChunkRows(ctx context.Context, chunkID int64) (json.RawMessage, error) {
	var rows []struct {
		Rows string `json:"rows"`
	}
	if err := r.client.Prisma.QueryRaw(chunkRowsQuery, strconv.FormatInt(chunkID, 10)).Exec(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read replay chunk %d: %w", chunkID, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("read replay chunk %d: %w", chunkID, errArchiveGone)
	}
	return compactJSON(rows[0].Rows)
}

// compactJSON removes the insignificant whitespace of a stored JSON value.
func compactJSON(stored string) (json.RawMessage, error) {
	var out bytes.Buffer
	if err := json.Compact(&out, []byte(stored)); err != nil {
		return nil, fmt.Errorf("stored replay chunk is not JSON: %w", err)
	}
	return out.Bytes(), nil
}

// Create starts an incomplete archive for the session.
func (r *ReplayCacheRepository) Create(ctx context.Context, sessionID string) (string, error) {
	archive, err := r.client.RaceArchive.CreateOne(
		db.RaceArchive.SessionID.Set(sessionID),
		db.RaceArchive.Metadata.Set(db.JSON(`{"complete":false}`)),
		db.RaceArchive.Counts.Set(db.JSON(`{}`)),
	).Exec(ctx)
	if err != nil {
		return "", fmt.Errorf("create replay archive: %w", err)
	}
	return archive.ID, nil
}

// SaveChunk stores one part of the archive. Linking to a deleted archive fails, which stops
// the recording of an invalidated dump.
func (r *ReplayCacheRepository) SaveChunk(ctx context.Context, archiveID string, index int, part domain.ReplayPart) error {
	optional := []db.RaceDatasetChunkSetParam{db.RaceDatasetChunk.ChunkIndex.Set(index)}
	if part.DriverNumber != nil {
		optional = append(optional, db.RaceDatasetChunk.DriverNumber.Set(*part.DriverNumber))
	}
	_, err := r.client.RaceDatasetChunk.CreateOne(
		db.RaceDatasetChunk.Dataset.Set(part.Dataset),
		db.RaceDatasetChunk.Rows.Set(db.JSON(part.Rows)),
		db.RaceDatasetChunk.Archive.Link(db.RaceArchive.ID.Equals(archiveID)),
		optional...,
	).Exec(ctx)
	if err != nil {
		return fmt.Errorf("save replay chunk %d: %w", index, err)
	}
	return nil
}

// MarkComplete makes the archive servable unless the session already has a complete archive
// (a concurrent recording finished first): it reports kept=false then, and the caller drops its
// own copy. A complete archive is never deleted here, so a client streaming it is never cut;
// both copies hold the same samples, since an ingestion in between would have deleted both.
// The advisory lock serialises two recordings completing at the same moment. Recordings
// abandoned more than an hour ago (process killed mid-dump) are cleaned up on the way.
func (r *ReplayCacheRepository) MarkComplete(ctx context.Context, archiveID string, sessionID string) (bool, error) {
	lock := r.client.Prisma.ExecuteRaw(lockSessionArchivesQuery, sessionID).Tx()
	complete := r.client.Prisma.ExecuteRaw(completeArchiveQuery, archiveID, sessionID).Tx()
	if err := r.client.Prisma.Transaction(lock, complete).Exec(ctx); err != nil {
		return false, fmt.Errorf("complete replay archive: %w", err)
	}
	if complete.Result().Count == 0 {
		return false, nil
	}
	if _, err := r.client.Prisma.ExecuteRaw(deleteStaleRecordings, sessionID, archiveID).Exec(ctx); err != nil {
		return true, fmt.Errorf("drop abandoned replay recordings: %w", err)
	}
	return true, nil
}

// Delete removes one archive and its parts.
func (r *ReplayCacheRepository) Delete(ctx context.Context, archiveID string) error {
	if _, err := r.client.Prisma.ExecuteRaw(deleteArchiveQuery, archiveID).Exec(ctx); err != nil {
		return fmt.Errorf("delete replay archive: %w", err)
	}
	return nil
}

// Invalidate removes every archive of the session (its samples changed).
func (r *ReplayCacheRepository) Invalidate(ctx context.Context, sessionID string) error {
	if _, err := r.client.Prisma.ExecuteRaw(deleteSessionArchives, sessionID).Exec(ctx); err != nil {
		return fmt.Errorf("invalidate replay archives: %w", err)
	}
	return nil
}
