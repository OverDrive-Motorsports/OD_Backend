/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed.repository.go - Prisma-backed read/replace of SessionDriverBroadcast.feeds.
##
*/

package prismaadapter

import (
	"context"
	"fmt"

	db "overdrive/services/race-data-service/resources/db"
	"overdrive/services/race-data-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

type DriverFeedRepository struct {
	client *db.PrismaClient
}

// NewDriverFeedRepository builds and returns a driver feed repository with its required dependencies.
func NewDriverFeedRepository(client *db.PrismaClient) *DriverFeedRepository {
	return &DriverFeedRepository{client: client}
}

// driverBroadcastID derives the SessionDriverBroadcast.driverId key the same way ingestion does.
func driverBroadcastID(driverNumber int) string {
	return contracts.DriverID("openf1", "f1", fmt.Sprintf("%d", driverNumber))
}

// loadDriverFeeds reads the feed list of one session+driver row; an absent row is an empty list.
func loadDriverFeeds(ctx context.Context, client *db.PrismaClient, sessionID string, driverNumber int) ([]domain.Feed, error) {
	row, err := client.SessionDriverBroadcast.FindUnique(
		db.SessionDriverBroadcast.SessionIDDriverID(
			db.SessionDriverBroadcast.SessionID.Equals(sessionID),
			db.SessionDriverBroadcast.DriverID.Equals(driverBroadcastID(driverNumber)),
		),
	).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return []domain.Feed{}, nil
		}
		return nil, err
	}
	if row == nil {
		return []domain.Feed{}, nil
	}
	return decodeFeeds(row.Feeds), nil
}

// GetDriverFeeds returns the stored feed list, or an empty list when no row exists yet.
func (r *DriverFeedRepository) GetDriverFeeds(ctx context.Context, sessionID string, driverNumber int) ([]domain.Feed, error) {
	return loadDriverFeeds(ctx, r.client, sessionID, driverNumber)
}

// ReplaceDriverFeeds upserts the session+driver row with the supplied list (creating the row
// when ingestion has not seeded it yet) and returns the stored list.
func (r *DriverFeedRepository) ReplaceDriverFeeds(ctx context.Context, sessionID string, driverNumber int, feeds []domain.Feed) ([]domain.Feed, error) {
	payload, err := encodeFeeds(feeds)
	if err != nil {
		return nil, err
	}
	driverID := driverBroadcastID(driverNumber)
	row, err := r.client.SessionDriverBroadcast.UpsertOne(
		db.SessionDriverBroadcast.SessionIDDriverID(
			db.SessionDriverBroadcast.SessionID.Equals(sessionID),
			db.SessionDriverBroadcast.DriverID.Equals(driverID),
		),
	).Create(
		db.SessionDriverBroadcast.SessionID.Set(sessionID),
		db.SessionDriverBroadcast.DriverID.Set(driverID),
		db.SessionDriverBroadcast.Feeds.Set(payload),
	).Update(
		db.SessionDriverBroadcast.Feeds.Set(payload),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return feeds, nil
	}
	return decodeFeeds(row.Feeds), nil
}
