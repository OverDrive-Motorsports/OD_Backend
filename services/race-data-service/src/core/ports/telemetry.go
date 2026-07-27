/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## telemetry.go - Package ports source file for services/race-data-service/src/core/ports.
	##
*/

package ports

import (
	"context"

	"overdrive/services/race-data-service/src/core/domain"
)

// TelemetryRepository reads live telemetry data (speed, engine, location, intervals)
// from the service's own Prisma-backed store.
type TelemetryRepository interface {
	GetSpeed(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetrySpeed, error)
	GetEngine(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryEngine, error)
	ListLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryLocation, error)
	GetIntervals(ctx context.Context, sessionID string, driverNumber int) (*domain.TelemetryIntervals, error)
}

// TelemetryUseCase exposes the GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/* application logic.
type TelemetryUseCase interface {
	GetSpeed(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetrySpeed, error)
	GetEngine(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryEngine, error)
	GetLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryLocation, error)
	GetIntervals(ctx context.Context, sessionID string, driverNumber int) (*domain.TelemetryIntervals, error)
	SessionExists(ctx context.Context, sessionID string) (bool, error)
}
