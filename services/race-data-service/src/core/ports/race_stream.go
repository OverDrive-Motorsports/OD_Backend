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
	"encoding/json"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// RaceStreamRepository reads session-scoped historical samples, oldest first, used to build
// the GET /sessions/{sessionId}/race/replay dump one driver at a time, so memory stays bounded
// by one driver's samples whatever the session size.
//
// Track-wide datasets (race control, weather) take no driver. Every method keeps only the
// samples inside window (an empty window keeps everything).
type RaceStreamRepository interface {
	// ListReplayDriverNumbers returns every driver number with at least one driver-scoped sample
	// (telemetry, position, radio, pit stop, stint or lap) in the session.
	ListReplayDriverNumbers(ctx context.Context, sessionID string) ([]int, error)
	ListTelemetryFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]domain.RaceReplayTelemetryFrame, error)
	ListPositionFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]domain.RacePosition, error)
	ListRaceControlFrames(ctx context.Context, sessionID string, window domain.TimeWindow) ([]domain.RaceControlEvent, error)
	ListWeatherFrames(ctx context.Context, sessionID string, window domain.TimeWindow) ([]domain.RaceWeatherSample, error)
	ListRadioFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]domain.RaceRadioMessage, error)
	ListPitStopFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]RaceReplayPitStopFrame, error)
	ListStintFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]RaceReplayStintFrame, error)
	ListLapFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]RaceReplayLapFrame, error)
}

// ReplayChunkRef identifies one cached part without loading its rows.
type ReplayChunkRef struct {
	ID           int64
	Dataset      string
	DriverNumber *int
}

// ReplayCache stores the parts of a full replay dump (RaceArchive + RaceDatasetChunk) so a
// session is built from the sample tables once, then served chunk by chunk. An archive is only
// served once marked complete; any ingestion into the session deletes its archives.
type ReplayCache interface {
	// FindComplete returns the newest complete archive of the session.
	FindComplete(ctx context.Context, sessionID string) (archiveID string, found bool, err error)
	// ListChunks returns the archive's parts in write order, without their rows.
	ListChunks(ctx context.Context, archiveID string) ([]ReplayChunkRef, error)
	// ChunkRows returns the JSON array of one part.
	ChunkRows(ctx context.Context, chunkID int64) (json.RawMessage, error)
	// Create starts an incomplete archive for the session.
	Create(ctx context.Context, sessionID string) (archiveID string, err error)
	// SaveChunk stores one part at position index of the archive. It fails once the archive
	// has been deleted (invalidated by an ingestion).
	SaveChunk(ctx context.Context, archiveID string, index int, part domain.ReplayPart) error
	// MarkComplete makes the archive servable. It reports kept=false (no error) when the
	// session already has a complete archive or this one was deleted meanwhile: the caller
	// then drops its own copy. A complete archive is never deleted by it.
	MarkComplete(ctx context.Context, archiveID string, sessionID string) (kept bool, err error)
	// Delete removes one archive and its parts.
	Delete(ctx context.Context, archiveID string) error
	ReplayCacheInvalidator
}

// ReplayCacheInvalidator drops the cached replays of a session whose data changed.
type ReplayCacheInvalidator interface {
	Invalidate(ctx context.Context, sessionID string) error
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
// lapStart/lapEnd/compound/tyreAgeLapsStart) - there is no "moment a stint
// happened" recorded anywhere. The chosen approximation anchors each stint to
// the recorded start time of its lapStart lap (RaceLap.dateStartUtc), the
// same lap-window-matching idea used elsewhere in this file for telemetry's
// derived lapNumber. This is a best-effort placement for sorting purposes
// only, NOT a real recorded event time - a stint whose lapStart lap has no
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

// RaceReplayUseCase exposes the GET /sessions/{sessionId}/race/replay application logic: the
// dump of a session's telemetry/position/radio/pitStop/stint/lap samples (grouped by driver) and
// its track-wide raceControl/weather samples, produced part by part.
//
// This is a REPLAY of already-ingested historical samples for a finished/stored session, NOT a
// live tail of an in-progress race.
type RaceReplayUseCase interface {
	SessionExists(ctx context.Context, sessionID string) (bool, error)
	// StreamReplay calls emit once per part, in dump order. It returns domain.ErrReplayBusy
	// before emitting anything when the dump must be built from the sample tables and the
	// concurrent build cap is reached (a cached dump is served without a slot).
	StreamReplay(ctx context.Context, sessionID string, query domain.ReplayQuery, emit func(domain.ReplayPart) error) error
}
