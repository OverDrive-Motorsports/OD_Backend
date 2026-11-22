/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_control_notification.repository.go - Postgres store of race control batches shared by the instances.
##
*/

package prismaadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	db "overdrive/services/race-data-service/resources/db"
	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// The queries stamp and compare rows with the database clock only (now()), so instances whose
// clocks drift still agree on the poll window. id and events are cast to text because the raw
// query result does not decode BigInt and jsonb into Go types reliably.
const (
	insertRaceControlNotificationQuery = `INSERT INTO "RaceControlNotification" ("sessionId", "events", "createdAt") VALUES ($1::text, $2::jsonb, now())`
	listRecentRaceControlQuery         = `SELECT "id"::text AS "id", "sessionId", "events"::text AS "events" FROM "RaceControlNotification" WHERE "createdAt" > now() - make_interval(secs => $1::int) ORDER BY "id"`
	deleteOldRaceControlQuery          = `DELETE FROM "RaceControlNotification" WHERE "createdAt" < now() - make_interval(secs => $1::int)`
)

// RaceControlNotificationRepository implements ports.RaceControlNotificationStore.
type RaceControlNotificationRepository struct {
	client *db.PrismaClient
}

// NewRaceControlNotificationRepository builds the store on the race-data database.
func NewRaceControlNotificationRepository(client *db.PrismaClient) *RaceControlNotificationRepository {
	return &RaceControlNotificationRepository{client: client}
}

// Insert stores one race control batch.
func (r *RaceControlNotificationRepository) Insert(ctx context.Context, sessionID string, events []domain.RaceControlEvent) error {
	payload, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("encode race control batch: %w", err)
	}
	if _, err := r.client.Prisma.ExecuteRaw(insertRaceControlNotificationQuery, sessionID, string(payload)).Exec(ctx); err != nil {
		return fmt.Errorf("insert race control notification: %w", err)
	}
	return nil
}

// ListRecent returns the batches stored during the last window, oldest first.
func (r *RaceControlNotificationRepository) ListRecent(ctx context.Context, window time.Duration) ([]ports.RaceControlNotification, error) {
	var rows []struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionId"`
		Events    string `json:"events"`
	}
	if err := r.client.Prisma.QueryRaw(listRecentRaceControlQuery, wholeSeconds(window)).Exec(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list race control notifications: %w", err)
	}
	return decodeRaceControlNotifications(rows)
}

// DeleteOlderThan removes the batches stored more than age ago.
func (r *RaceControlNotificationRepository) DeleteOlderThan(ctx context.Context, age time.Duration) error {
	if _, err := r.client.Prisma.ExecuteRaw(deleteOldRaceControlQuery, wholeSeconds(age)).Exec(ctx); err != nil {
		return fmt.Errorf("delete old race control notifications: %w", err)
	}
	return nil
}

// wholeSeconds rounds a duration up to whole seconds, bound as an integer parameter.
func wholeSeconds(d time.Duration) int {
	return int((d + time.Second - 1) / time.Second)
}

// decodeRaceControlNotifications turns the raw rows into notifications.
func decodeRaceControlNotifications(rows []struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Events    string `json:"events"`
}) ([]ports.RaceControlNotification, error) {
	items := make([]ports.RaceControlNotification, 0, len(rows))
	for _, row := range rows {
		id, err := strconv.ParseInt(row.ID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("race control notification id %q: %w", row.ID, err)
		}
		var events []domain.RaceControlEvent
		if err := json.Unmarshal([]byte(row.Events), &events); err != nil {
			return nil, fmt.Errorf("race control notification %d: %w", id, err)
		}
		items = append(items, ports.RaceControlNotification{ID: id, SessionID: row.SessionID, Events: events})
	}
	return items, nil
}
