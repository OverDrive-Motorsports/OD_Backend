/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## telemetry.usecase.go - Package usecases source file for services/race-data-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

type TelemetryUseCase struct {
	repository ports.TelemetryRepository
	client     ports.ChampionshipClient
}

// NewTelemetryUseCase builds and returns a telemetry use case with its required dependencies.
func NewTelemetryUseCase(repository ports.TelemetryRepository, client ports.ChampionshipClient) *TelemetryUseCase {
	return &TelemetryUseCase{repository: repository, client: client}
}

// SessionExists reports whether sessionID is a known session in championship-service.
func (u *TelemetryUseCase) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	session, err := u.client.GetSession(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return session != nil, nil
}

// GetSpeed returns every speed sample for a driver, oldest first.
func (u *TelemetryUseCase) GetSpeed(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetrySpeed, error) {
	return u.repository.GetSpeed(ctx, sessionID, driverNumber, lapNumber)
}

// GetEngine returns every engine sample for a driver, oldest first.
func (u *TelemetryUseCase) GetEngine(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryEngine, error) {
	return u.repository.GetEngine(ctx, sessionID, driverNumber, lapNumber)
}

// GetLocation returns spatial samples for a driver.
func (u *TelemetryUseCase) GetLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryLocation, error) {
	return u.repository.ListLocation(ctx, sessionID, driverNumber, lapNumber)
}

// GetIntervals returns the latest interval telemetry for a driver.
func (u *TelemetryUseCase) GetIntervals(ctx context.Context, sessionID string, driverNumber int) (*domain.TelemetryIntervals, error) {
	return u.repository.GetIntervals(ctx, sessionID, driverNumber)
}
