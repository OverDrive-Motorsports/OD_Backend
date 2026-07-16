/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_live.usecase.go - Package usecases source file for services/race-data-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

type RaceLiveUseCase struct {
	repository  ports.RaceLiveRepository
	client      ports.ChampionshipClient
	broadcaster ports.RaceControlBroadcaster
}

// NewRaceLiveUseCase builds and returns a race live use case with its required dependencies.
func NewRaceLiveUseCase(repository ports.RaceLiveRepository, client ports.ChampionshipClient, broadcaster ports.RaceControlBroadcaster) *RaceLiveUseCase {
	return &RaceLiveUseCase{repository: repository, client: client, broadcaster: broadcaster}
}

// SessionExists reports whether sessionID is a known session in championship-service.
// It backs the "unknown sessionId -> 404" contract for every /race/* and
// /telemetry/* endpoint.
func (u *RaceLiveUseCase) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	session, err := u.client.GetSession(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return session != nil, nil
}

// GetPosition returns a single driver's live position when driverNumber is given,
// or the bare array of all drivers' positions otherwise.
func (u *RaceLiveUseCase) GetPosition(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) (any, error) {
	items, err := u.repository.ListPositions(ctx, sessionID, driverNumber, lapNumber)
	if err != nil {
		return nil, err
	}
	if driverNumber != nil {
		if len(items) == 0 {
			return nil, nil
		}
		return items[0], nil
	}
	return items, nil
}

// GetLaps returns a single driver's laps object when driverNumber is given,
// or the bare array of every driver's laps object otherwise.
func (u *RaceLiveUseCase) GetLaps(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) (any, error) {
	if driverNumber != nil {
		response, err := u.repository.ListLapsForDriver(ctx, sessionID, *driverNumber, lapNumber)
		if err != nil {
			return nil, err
		}
		if len(response.Laps) == 0 && response.BestLap == nil {
			return nil, nil
		}
		return response, nil
	}
	drivers, err := u.repository.ListDriverNumbers(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RaceLapsResponse, 0, len(drivers))
	for _, driver := range drivers {
		response, err := u.repository.ListLapsForDriver(ctx, sessionID, driver, lapNumber)
		if err != nil {
			return nil, err
		}
		items = append(items, response)
	}
	return items, nil
}

// GetStints returns tyre stints, optionally filtered by driver.
func (u *RaceLiveUseCase) GetStints(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceStint, error) {
	return u.repository.ListStints(ctx, sessionID, driverNumber)
}

// GetPitStops returns pit stops, optionally filtered by driver.
func (u *RaceLiveUseCase) GetPitStops(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePitStop, error) {
	return u.repository.ListPitStops(ctx, sessionID, driverNumber)
}

// GetWeather returns all weather samples for a session.
func (u *RaceLiveUseCase) GetWeather(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error) {
	return u.repository.ListWeather(ctx, sessionID)
}

// GetRadio returns team radio messages, optionally filtered by driver.
func (u *RaceLiveUseCase) GetRadio(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error) {
	return u.repository.ListRadio(ctx, sessionID, driverNumber)
}

// WaitForRaceControl blocks (long-polls) until a new race control event batch is
// published for sessionID or timeout elapses, then returns. The HTTP handler
// closes the response after this call returns — the client must re-POST to
// receive the next event. See race_control_broadcaster.go for the single-instance
// in-memory caveat.
func (u *RaceLiveUseCase) WaitForRaceControl(ctx context.Context, sessionID string, timeout time.Duration) ([]domain.RaceControlEvent, error) {
	events := u.broadcaster.Wait(ctx, sessionID, timeout)
	return events, nil
}
