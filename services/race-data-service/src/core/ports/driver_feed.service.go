/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed.service.go - Use case port for reading/replacing a driver's onboard feed list.
##
*/

package ports

import (
	"context"

	"overdrive/services/race-data-service/src/core/domain"
)

// DriverFeedUseCase backs GET/PUT /sessions/{sessionId}/drivers/{driverNumber}/broadcast.
// Both methods return domain.ErrSessionNotFound / domain.ErrDriverNotFound when
// championship-service does not know the session or the driver is not in its driver list,
// domain.ErrChampionshipUnavailable when that lookup fails, and ReplaceDriverFeeds returns
// domain.ErrInvalidFeed (wrapped) when the list violates domain.ValidateFeeds.
type DriverFeedUseCase interface {
	GetDriverFeeds(ctx context.Context, sessionID string, driverNumber int) (*domain.DriverBroadcast, error)
	ReplaceDriverFeeds(ctx context.Context, sessionID string, driverNumber int, feeds []domain.Feed) (*domain.DriverBroadcast, error)
}
