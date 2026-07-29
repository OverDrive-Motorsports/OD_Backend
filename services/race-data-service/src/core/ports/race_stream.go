/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_stream.go - Package ports source file for services/race-data-service/src/core/ports.
	##
*/

package ports

import (
	"context"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// RaceStreamRepository reads session-scoped historical samples, oldest first,
// used to build the GET /sessions/{sessionId}/race/replay bulk dump.
//
// weather is track-wide like race control (no driver filter, matching
// GET /race/weather not taking a driverNumber param either); radio/pitStop/
// stint/lap are driver-scoped data, filtered the same way telemetry/position
// already are when driverNumber is given.
type RaceStreamRepository interface {
	ListTelemetryFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceReplayTelemetryFrame, error)
	ListPositionFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePosition, error)
	ListRaceControlFrames(ctx context.Context, sessionID string) ([]domain.RaceControlEvent, error)
	ListWeatherFrames(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error)
	ListRadioFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error)
	ListPitStopFrames(ctx context.Context, sessionID string, driverNumber *int) ([]RaceReplayPitStopFrame, error)
	ListStintFrames(ctx context.Context, sessionID string, driverNumber *int) ([]RaceReplayStintFrame, error)
	ListLapFrames(ctx context.Context, sessionID string, driverNumber *int) ([]RaceReplayLapFrame, error)
}

// RaceReplayPitStopFrame pairs a pit stop payload with the dateUtc used to
// sort it within its driver's array. domain.RacePitStop's JSON shape
// intentionally has no timestamp field (it matches the GET /race/pitstops
// row exactly), so the timestamp used for ordering travels alongside the
// payload here instead of inside it.
type RaceReplayPitStopFrame struct {
	Payload   domain.RacePitStop
	Timestamp time.Time
}

// RaceReplayStintFrame pairs a stint payload with an APPROXIMATED timestamp.
// RaceStint has no timestamp column at all in the schema (only stintNumber/
// lapStart/lapEnd/compound/tyreAgeLapsStart) — there is no "moment a stint
// happened" recorded anywhere. The chosen approximation anchors each stint to
// the recorded start time of its lapStart lap (RaceLap.dateStartUtc), the
// same lap-window-matching idea used elsewhere in this file for telemetry's
// derived lapNumber. This is a best-effort placement for sorting purposes
// only, NOT a real recorded event time — a stint whose lapStart lap has no
// recorded dateStartUtc is skipped entirely rather than guessed at.
type RaceReplayStintFrame struct {
	Payload   domain.RaceStint
	Timestamp time.Time
}

// RaceReplayLapFrame pairs a domain.RaceReplayLapEvent payload with its
// completion time (dateStartUtc + lapDurationSec). Lap timing (duration,
// sectors) is only known once the lap has actually finished, so the event is
// placed at completion rather than at lap start. Laps missing either
// dateStartUtc or lapDurationSec (both nullable in the schema) are skipped
// rather than emitted with a guessed or zero timestamp.
type RaceReplayLapFrame struct {
	Payload   domain.RaceReplayLapEvent
	Timestamp time.Time
}

// RaceReplayUseCase exposes the GET /sessions/{sessionId}/race/replay
// application logic: it builds a plain JSON bulk dump of a session's
// telemetry/position/radio/pitStop/stint/lap samples (grouped by driver) plus
// its track-wide raceControl/weather samples (flat arrays).
//
// This is a REPLAY of already-ingested historical samples for a
// finished/stored session — NOT a live tail of an in-progress race (there is
// no live race happening in this dev setup; all data is historical).
type RaceReplayUseCase interface {
	SessionExists(ctx context.Context, sessionID string) (bool, error)
	GetReplay(ctx context.Context, sessionID string, driverNumber *int) (domain.RaceReplay, error)
}
