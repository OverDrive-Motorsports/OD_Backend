/**
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
	// GetSpeed, GetEngine and ListLocation read page.Fetch() rows from page.Offset, oldest first.
	GetSpeed(ctx context.Context, sessionID string, driverNumber int, lapNumber *int, page domain.Page) ([]domain.TelemetrySpeed, error)
	GetEngine(ctx context.Context, sessionID string, driverNumber int, lapNumber *int, page domain.Page) ([]domain.TelemetryEngine, error)
	ListLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber *int, page domain.Page) ([]domain.TelemetryLocation, error)
	GetIntervals(ctx context.Context, sessionID string, driverNumber int) (*domain.TelemetryIntervals, error)
}

// TelemetryUseCase exposes the GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/* application logic.
type TelemetryUseCase interface {
	// GetSpeed, GetEngine and GetLocation return one page and the next page offset, nil on the
	// last page.
	GetSpeed(ctx context.Context, sessionID string, driverNumber int, lapNumber *int, page domain.Page) ([]domain.TelemetrySpeed, *int, error)
	GetEngine(ctx context.Context, sessionID string, driverNumber int, lapNumber *int, page domain.Page) ([]domain.TelemetryEngine, *int, error)
	GetLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber *int, page domain.Page) ([]domain.TelemetryLocation, *int, error)
	GetIntervals(ctx context.Context, sessionID string, driverNumber int) (*domain.TelemetryIntervals, error)
	SessionExists(ctx context.Context, sessionID string) (bool, error)
}
