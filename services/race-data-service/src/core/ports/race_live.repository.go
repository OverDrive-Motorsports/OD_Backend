/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_live.repository.go - Package ports source file for services/race-data-service/src/core/ports.
	##
*/

package ports

import (
	"context"

	"overdrive/services/race-data-service/src/core/domain"
)

// RaceLiveRepository reads live race data (position, laps, stints, pitstops,
// weather, radio) from the service's own Prisma-backed store.
type RaceLiveRepository interface {
	ListPositions(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) ([]domain.RacePosition, error)
	ListDriverNumbers(ctx context.Context, sessionID string) ([]int, error)
	ListLapsForDriver(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) (domain.RaceLapsResponse, error)
	ListStints(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceStint, error)
	ListPitStops(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePitStop, error)
	ListWeather(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error)
	ListRadio(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error)
}
