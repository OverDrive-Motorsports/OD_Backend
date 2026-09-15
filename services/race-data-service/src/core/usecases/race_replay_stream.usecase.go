/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_replay_stream.usecase.go - Package usecases source file for services/race-data-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"strconv"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// RaceReplayUseCase implements ports.RaceReplayUseCase.
type RaceReplayUseCase struct {
	repository ports.RaceStreamRepository
	client     ports.ChampionshipClient
}

// NewRaceReplayStreamUseCase builds and returns a race replay use case with its required dependencies.
func NewRaceReplayStreamUseCase(repository ports.RaceStreamRepository, client ports.ChampionshipClient) *RaceReplayUseCase {
	return &RaceReplayUseCase{repository: repository, client: client}
}

// SessionExists reports whether sessionID is a known session in championship-service.
// It backs the "unknown sessionId -> 404" contract shared with every other
// /race/* and /telemetry/* endpoint.
func (u *RaceReplayUseCase) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	session, err := u.client.GetSession(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return session != nil, nil
}

// GetReplay loads the session's telemetry/position/raceControl/weather/radio/
// pitStop/stint/lap samples and assembles the plain JSON bulk-dump response:
// driver-scoped datasets grouped into a map keyed by driver number, track-wide
// datasets (raceControl/weather) left as flat arrays. Each repository
// List*Frames call already returns its rows oldest-first, so no additional
// merging/sorting across dataset types is needed here — every dataset lives
// under its own top-level key rather than one interleaved timeline.
func (u *RaceReplayUseCase) GetReplay(ctx context.Context, sessionID string, driverNumber *int) (domain.RaceReplay, error) {
	telemetry, err := u.repository.ListTelemetryFrames(ctx, sessionID, driverNumber)
	if err != nil {
		return domain.RaceReplay{}, err
	}
	positions, err := u.repository.ListPositionFrames(ctx, sessionID, driverNumber)
	if err != nil {
		return domain.RaceReplay{}, err
	}
	// Race control alerts (flags/safety car/track status) are track-wide, not
	// per-driver — they are always included regardless of driverNumber.
	raceControl, err := u.repository.ListRaceControlFrames(ctx, sessionID)
	if err != nil {
		return domain.RaceReplay{}, err
	}
	// Weather is track-wide too, matching GET /race/weather not taking a
	// driverNumber param either.
	weather, err := u.repository.ListWeatherFrames(ctx, sessionID)
	if err != nil {
		return domain.RaceReplay{}, err
	}
	radio, err := u.repository.ListRadioFrames(ctx, sessionID, driverNumber)
	if err != nil {
		return domain.RaceReplay{}, err
	}
	pitStops, err := u.repository.ListPitStopFrames(ctx, sessionID, driverNumber)
	if err != nil {
		return domain.RaceReplay{}, err
	}
	// Stint timestamps are approximated — see ports.RaceReplayStintFrame and
	// race_stream.repository.go's ListStintFrames for why. Approximated or
	// not, they're only used here to keep each driver's stint array in order.
	stints, err := u.repository.ListStintFrames(ctx, sessionID, driverNumber)
	if err != nil {
		return domain.RaceReplay{}, err
	}
	laps, err := u.repository.ListLapFrames(ctx, sessionID, driverNumber)
	if err != nil {
		return domain.RaceReplay{}, err
	}

	replay := domain.RaceReplay{
		Telemetry:   map[string][]domain.RaceReplayTelemetrySample{},
		Position:    map[string][]domain.RaceReplayPositionSample{},
		Radio:       map[string][]domain.RaceReplayRadioSample{},
		PitStop:     map[string][]domain.RaceReplayPitStopSample{},
		Stint:       map[string][]domain.RaceReplayStintSample{},
		Lap:         map[string][]domain.RaceReplayLapSample{},
		RaceControl: raceControl,
		Weather:     weather,
	}

	for _, frame := range telemetry {
		key := strconv.Itoa(frame.DriverNumber)
		replay.Telemetry[key] = append(replay.Telemetry[key], domain.RaceReplayTelemetrySample{
			Speed:           frame.Speed,
			Rpm:             frame.Rpm,
			Gear:            frame.Gear,
			ThrottlePercent: frame.ThrottlePercent,
			BrakePercent:    frame.BrakePercent,
			DrsActive:       frame.DrsActive,
			LapNumber:       frame.LapNumber,
			X:               frame.X,
			Y:               frame.Y,
			Z:               frame.Z,
			Timestamp:       frame.Timestamp,
		})
	}
	for _, position := range positions {
		key := strconv.Itoa(position.DriverNumber)
		replay.Position[key] = append(replay.Position[key], domain.RaceReplayPositionSample{
			Position:      position.Position,
			GapToLeader:   position.GapToLeader,
			GapAhead:      position.GapAhead,
			GapBehind:     position.GapBehind,
			LapsCompleted: position.LapsCompleted,
			Timestamp:     position.Timestamp,
		})
	}
	for _, message := range radio {
		key := strconv.Itoa(message.DriverNumber)
		replay.Radio[key] = append(replay.Radio[key], domain.RaceReplayRadioSample{
			Timestamp: message.Timestamp,
			AudioURL:  message.AudioURL,
		})
	}
	for _, frame := range pitStops {
		key := strconv.Itoa(frame.Payload.DriverNumber)
		replay.PitStop[key] = append(replay.PitStop[key], domain.RaceReplayPitStopSample{
			LapNumber:   frame.Payload.LapNumber,
			PitDuration: frame.Payload.PitDuration,
		})
	}
	for _, frame := range stints {
		key := strconv.Itoa(frame.Payload.DriverNumber)
		replay.Stint[key] = append(replay.Stint[key], domain.RaceReplayStintSample{
			StintNumber:    frame.Payload.StintNumber,
			Compound:       frame.Payload.Compound,
			LapStart:       frame.Payload.LapStart,
			LapEnd:         frame.Payload.LapEnd,
			TyreAgeAtStart: frame.Payload.TyreAgeAtStart,
		})
	}
	for _, frame := range laps {
		key := strconv.Itoa(frame.Payload.DriverNumber)
		replay.Lap[key] = append(replay.Lap[key], domain.RaceReplayLapSample{
			LapNumber:   frame.Payload.LapNumber,
			LapDuration: frame.Payload.LapDuration,
			Sector1:     frame.Payload.Sector1,
			Sector2:     frame.Payload.Sector2,
			Sector3:     frame.Payload.Sector3,
			IsPitOutLap: frame.Payload.IsPitOutLap,
		})
	}

	return replay, nil
}
