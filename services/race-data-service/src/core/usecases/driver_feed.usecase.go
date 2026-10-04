/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed.usecase.go - Validates and persists a driver's onboard feed list after checking the driver exists.
##
*/

package usecases

import (
	"context"
	"fmt"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

type DriverFeedUseCase struct {
	repository ports.DriverFeedRepository
	client     ports.ChampionshipClient
}

// NewDriverFeedUseCase builds and returns a driver feed use case with its required dependencies.
func NewDriverFeedUseCase(repository ports.DriverFeedRepository, client ports.ChampionshipClient) *DriverFeedUseCase {
	return &DriverFeedUseCase{repository: repository, client: client}
}

// GetDriverFeeds returns the driver's feed list once the session and driver are confirmed to
// exist in championship-service; a driver with no stored row yields an empty list.
func (u *DriverFeedUseCase) GetDriverFeeds(ctx context.Context, sessionID string, driverNumber int) (*domain.DriverBroadcast, error) {
	if err := u.ensureDriver(ctx, sessionID, driverNumber); err != nil {
		return nil, err
	}
	feeds, err := u.repository.GetDriverFeeds(ctx, sessionID, driverNumber)
	if err != nil {
		return nil, err
	}
	return newDriverBroadcast(sessionID, driverNumber, feeds), nil
}

// ReplaceDriverFeeds validates the list with domain.ValidateFeeds, confirms the session/driver
// exist, then replaces the whole stored list. A nil list is normalised to an empty one.
func (u *DriverFeedUseCase) ReplaceDriverFeeds(ctx context.Context, sessionID string, driverNumber int, feeds []domain.Feed) (*domain.DriverBroadcast, error) {
	if err := domain.ValidateFeeds(feeds); err != nil {
		return nil, err
	}
	if err := u.ensureDriver(ctx, sessionID, driverNumber); err != nil {
		return nil, err
	}
	if feeds == nil {
		feeds = []domain.Feed{}
	}
	stored, err := u.repository.ReplaceDriverFeeds(ctx, sessionID, driverNumber, feeds)
	if err != nil {
		return nil, err
	}
	return newDriverBroadcast(sessionID, driverNumber, stored), nil
}

// ensureDriver resolves the session's driver list from championship-service and maps the
// outcome to the domain sentinels: unknown session, driver not in session, or downstream failure.
func (u *DriverFeedUseCase) ensureDriver(ctx context.Context, sessionID string, driverNumber int) error {
	drivers, err := u.client.ListSessionDrivers(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrChampionshipUnavailable, err)
	}
	if drivers == nil {
		return domain.ErrSessionNotFound
	}
	for _, driver := range drivers {
		if driver.DriverNumber == driverNumber {
			return nil
		}
	}
	return domain.ErrDriverNotFound
}

// newDriverBroadcast builds the response payload, guaranteeing a non-nil feed list.
func newDriverBroadcast(sessionID string, driverNumber int, feeds []domain.Feed) *domain.DriverBroadcast {
	if feeds == nil {
		feeds = []domain.Feed{}
	}
	return &domain.DriverBroadcast{SessionID: sessionID, DriverNumber: driverNumber, Feeds: feeds}
}
