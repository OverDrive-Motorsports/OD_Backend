/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed.repository.go - Persistence port for a driver's onboard feed list (SessionDriverBroadcast).
##
*/

package ports

import (
	"context"

	"overdrive/services/race-data-service/src/core/domain"
)

// DriverFeedRepository reads and replaces the feed list stored on the SessionDriverBroadcast
// row of one session+driver. GetDriverFeeds returns an empty (non-nil) list when no row exists;
// ReplaceDriverFeeds upserts the row. Session/driver existence is the usecase's concern.
type DriverFeedRepository interface {
	GetDriverFeeds(ctx context.Context, sessionID string, driverNumber int) ([]domain.Feed, error)
	ReplaceDriverFeeds(ctx context.Context, sessionID string, driverNumber int, feeds []domain.Feed) ([]domain.Feed, error)
}
