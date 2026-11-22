/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_live.service.go - Package ports source file for services/race-data-service/src/core/ports.
##
*/

package ports

import (
	"context"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// RaceLiveUseCase exposes the GET /sessions/{sessionId}/race/* application logic,
// plus the long-poll wait behind POST /sessions/{sessionId}/race/control.
type RaceLiveUseCase interface {
	GetPosition(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) (any, error)
	GetLaps(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) (any, error)
	GetStints(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceStint, error)
	GetPitStops(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePitStop, error)
	GetWeather(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error)
	GetRadio(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error)
	WaitForRaceControl(ctx context.Context, sessionID string, timeout time.Duration) ([]domain.RaceControlEvent, error)
	SessionExists(ctx context.Context, sessionID string) (bool, error)
}

// RaceControlBroadcaster backs the long-polling contract of POST
// /sessions/{sessionId}/race/control: Publish announces a race control batch for a session,
// Wait blocks one client until the next batch for that session (or its timeout). Production
// uses usecases.SharedRaceControlBroadcaster, which publishes through Postgres so a waiter on
// any race-data-service instance is woken; tests use the in-memory usecases.RaceControlBroadcaster.
type RaceControlBroadcaster interface {
	Publish(sessionID string, events []domain.RaceControlEvent)
	// Wait returns domain.ErrRaceControlBusy when the waiter caps are reached and
	// domain.ErrShuttingDown during shutdown; a timeout or a cancelled context returns (nil, nil).
	Wait(ctx context.Context, sessionID string, timeout time.Duration) ([]domain.RaceControlEvent, error)
}

// RaceControlNotification is one published race control batch as stored for every instance.
type RaceControlNotification struct {
	ID        int64
	SessionID string
	Events    []domain.RaceControlEvent
}

// RaceControlNotificationStore is the shared table the instances publish race control batches
// to and poll them from (no extra infrastructure: it lives in the race-data database).
type RaceControlNotificationStore interface {
	// Insert stores a batch, stamped with the database clock.
	Insert(ctx context.Context, sessionID string, events []domain.RaceControlEvent) error
	// ListRecent returns the batches stored during the last window (database clock), oldest first.
	ListRecent(ctx context.Context, window time.Duration) ([]RaceControlNotification, error)
	// DeleteOlderThan removes the batches stored more than age ago.
	DeleteOlderThan(ctx context.Context, age time.Duration) error
}
