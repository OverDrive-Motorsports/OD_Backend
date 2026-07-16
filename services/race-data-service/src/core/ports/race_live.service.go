/*
*

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

// RaceControlBroadcaster is an in-memory, single-instance pub/sub used to implement
// the long-polling contract of POST /sessions/{sessionId}/race/control. It assumes
// a single race-data-service instance (no horizontal scale, no external broker such
// as Redis) — see accept_ingestion_batch.usecase.go and race_control_broadcaster.go.
type RaceControlBroadcaster interface {
	Publish(sessionID string, events []domain.RaceControlEvent)
	Wait(ctx context.Context, sessionID string, timeout time.Duration) []domain.RaceControlEvent
}
