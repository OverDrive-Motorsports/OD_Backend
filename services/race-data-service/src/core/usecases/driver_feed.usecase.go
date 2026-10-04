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
	"time"

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
// exist, checks any startedAtUtc against the session window (loaded from championship-service
// only when needed), then replaces the whole stored list with start times in UTC. A nil list
// is normalised to an empty one.
func (u *DriverFeedUseCase) ReplaceDriverFeeds(ctx context.Context, sessionID string, driverNumber int, feeds []domain.Feed) (*domain.DriverBroadcast, error) {
	if err := domain.ValidateFeeds(feeds); err != nil {
		return nil, err
	}
	if err := u.ensureDriver(ctx, sessionID, driverNumber); err != nil {
		return nil, err
	}
	if domain.HasFeedStartTimes(feeds) {
		window, err := u.sessionWindow(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		if err := domain.ValidateFeedStartTimes(feeds, window); err != nil {
			return nil, err
		}
	}
	stored, err := u.repository.ReplaceDriverFeeds(ctx, sessionID, driverNumber, domain.NormalizeFeeds(feeds))
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

// sessionWindow loads the session bounds from championship-service. A bound that is missing or
// not RFC 3339 is left zero/nil: a zero Start disables the window check rather than rejecting
// a valid feed because of an upstream formatting issue.
func (u *DriverFeedUseCase) sessionWindow(ctx context.Context, sessionID string) (domain.SessionWindow, error) {
	session, err := u.client.GetSession(ctx, sessionID)
	if err != nil {
		return domain.SessionWindow{}, fmt.Errorf("%w: %w", domain.ErrChampionshipUnavailable, err)
	}
	if session == nil {
		return domain.SessionWindow{}, domain.ErrSessionNotFound
	}
	var window domain.SessionWindow
	if start, err := time.Parse(time.RFC3339, session.StartedAtUTC); err == nil {
		window.Start = start
	}
	if end, err := time.Parse(time.RFC3339, session.EndedAtUTC); err == nil {
		window.End = &end
	}
	return window, nil
}

// newDriverBroadcast builds the response payload, guaranteeing a non-nil feed list.
func newDriverBroadcast(sessionID string, driverNumber int, feeds []domain.Feed) *domain.DriverBroadcast {
	if feeds == nil {
		feeds = []domain.Feed{}
	}
	return &domain.DriverBroadcast{SessionID: sessionID, DriverNumber: driverNumber, Feeds: feeds}
}
