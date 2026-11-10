/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog.repository.go - Finds which sessions have a playable replay (telemetry + location samples).
##
*/

package prismaadapter

import (
	"context"
	"fmt"
	"strings"

	db "overdrive/services/race-data-service/resources/db"
)

// replayCatalogBatchSize bounds the number of bind parameters per query; a championship has a
// few hundred sessions at most, so this is one query in practice.
const replayCatalogBatchSize = 500

type ReplayCatalogRepository struct {
	client *db.PrismaClient
}

// NewReplayCatalogRepository builds and returns a replay catalog repository with its required dependencies.
func NewReplayCatalogRepository(client *db.PrismaClient) *ReplayCatalogRepository {
	return &ReplayCatalogRepository{client: client}
}

// SessionsWithReplay returns the subset of sessionIDs that have at least one telemetry sample
// AND one location sample, what GET /sessions/{sessionId}/race/replay needs to animate cars
// (without location every frame sits at x=y=z=0). The check is a raw EXISTS query because
// prisma-client-go has no DISTINCT/EXISTS and both sample tables hold millions of rows; each
// EXISTS is served by the (sessionId, dateUtc) index and stops at the first row.
func (r *ReplayCatalogRepository) SessionsWithReplay(ctx context.Context, sessionIDs []string) (map[string]struct{}, error) {
	found := make(map[string]struct{}, len(sessionIDs))
	for start := 0; start < len(sessionIDs); start += replayCatalogBatchSize {
		end := min(start+replayCatalogBatchSize, len(sessionIDs))
		batch := sessionIDs[start:end]
		params := make([]any, len(batch))
		for index, id := range batch {
			params[index] = id
		}
		var rows []struct {
			SessionID string `json:"sessionId"`
		}
		if err := r.client.Prisma.QueryRaw(sessionsWithReplayQuery(len(batch)), params...).Exec(ctx, &rows); err != nil {
			return nil, fmt.Errorf("query sessions with replay: %w", err)
		}
		for _, row := range rows {
			found[row.SessionID] = struct{}{}
		}
	}
	return found, nil
}

// sessionsWithReplayQuery builds the EXISTS query for n candidate session IDs bound as $1..$n.
// Only placeholders are interpolated, never values.
func sessionsWithReplayQuery(n int) string {
	values := make([]string, n)
	for index := range values {
		values[index] = fmt.Sprintf("($%d::text)", index+1)
	}
	return `SELECT candidate.id AS "sessionId" FROM (VALUES ` + strings.Join(values, ", ") + `) AS candidate(id)` +
		` WHERE EXISTS (SELECT 1 FROM "RaceTelemetrySample" t WHERE t."sessionId" = candidate.id)` +
		` AND EXISTS (SELECT 1 FROM "RaceLocationSample" l WHERE l."sessionId" = candidate.id)`
}
