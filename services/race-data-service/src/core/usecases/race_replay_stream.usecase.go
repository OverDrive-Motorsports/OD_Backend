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
	"encoding/json"
	"log"
	"sort"
	"strconv"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// RaceReplayUseCase implements ports.RaceReplayUseCase. It never holds more than one driver's
// samples of one dataset in memory, caps the number of replays built at once, and caches full
// dumps so a session is read from the sample tables once.
type RaceReplayUseCase struct {
	repository ports.RaceStreamRepository
	cache      ports.ReplayCache
	client     ports.ChampionshipClient
	slots      chan struct{}
	cacheSlots chan struct{}
}

// NewRaceReplayStreamUseCase builds the replay use case. cache may be nil (no caching);
// maxBuilds bounds the replays built from the sample tables at once and maxCacheReads the
// cached replays served at once (each at least 1).
func NewRaceReplayStreamUseCase(repository ports.RaceStreamRepository, cache ports.ReplayCache, client ports.ChampionshipClient, maxBuilds int, maxCacheReads int) *RaceReplayUseCase {
	return &RaceReplayUseCase{
		repository: repository,
		cache:      cache,
		client:     client,
		slots:      make(chan struct{}, max(maxBuilds, 1)),
		cacheSlots: make(chan struct{}, max(maxCacheReads, 1)),
	}
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

// StreamReplay produces the replay dump part by part (see domain.ReplayPart). Without a time
// window it serves a complete cached archive when one exists; otherwise it reads the sample
// tables driver by driver, and a full dump (no driver, no window) is recorded into the cache
// as it goes.
func (u *RaceReplayUseCase) StreamReplay(ctx context.Context, sessionID string, query domain.ReplayQuery, emit func(domain.ReplayPart) error) error {
	if query.Window.From != nil && query.Window.To != nil && query.Window.From.After(*query.Window.To) {
		return domain.ErrInvalidTimeWindow
	}
	if u.cache != nil && query.Window.IsZero() {
		served, err := u.streamFromCache(ctx, sessionID, query.DriverNumber, emit)
		if served || err != nil {
			return err
		}
	}
	return u.streamFromDatabase(ctx, sessionID, query, emit)
}

// streamFromCache serves the newest complete archive, filtered to driverNumber when given. It
// reports served=false (and no error) when there is no usable archive, so the caller reads the
// database instead; a cache failure before the first part is logged and treated the same way.
func (u *RaceReplayUseCase) streamFromCache(ctx context.Context, sessionID string, driverNumber *int, emit func(domain.ReplayPart) error) (bool, error) {
	archiveID, found, err := u.cache.FindComplete(ctx, sessionID)
	if err != nil {
		log.Printf("replay cache: lookup for session %s failed, reading the samples: %v", sessionID, err)
		return false, nil
	}
	if !found {
		return false, nil
	}
	// Cached reads have their own, larger cap: they do not load the database, but each one holds
	// a chunk in memory for as long as its client takes to read it.
	select {
	case u.cacheSlots <- struct{}{}:
	default:
		return true, domain.ErrReplayBusy
	}
	defer func() { <-u.cacheSlots }()
	chunks, err := u.cache.ListChunks(ctx, archiveID)
	if err != nil {
		log.Printf("replay cache: listing archive %s failed, reading the samples: %v", archiveID, err)
		return false, nil
	}
	for _, chunk := range chunks {
		if driverNumber != nil && chunk.DriverNumber != nil && *chunk.DriverNumber != *driverNumber {
			continue
		}
		rows, err := u.cache.ChunkRows(ctx, chunk.ID)
		if err != nil {
			return true, err
		}
		if err := emit(domain.ReplayPart{Dataset: chunk.Dataset, DriverNumber: chunk.DriverNumber, Rows: rows}); err != nil {
			return true, err
		}
	}
	return true, nil
}

// streamFromDatabase reads every dataset one driver at a time and emits each non-empty part. It
// holds one of the concurrent replay slots (domain.ErrReplayBusy when none is free): building a
// dump is what loads the database. Serving a cached archive takes a slot of the separate cache
// cap instead, so slow readers of the cache never lock builds out.
func (u *RaceReplayUseCase) streamFromDatabase(ctx context.Context, sessionID string, query domain.ReplayQuery, emit func(domain.ReplayPart) error) error {
	select {
	case u.slots <- struct{}{}:
	default:
		return domain.ErrReplayBusy
	}
	defer func() { <-u.slots }()

	var drivers []int
	if query.DriverNumber != nil {
		drivers = []int{*query.DriverNumber}
	} else {
		var err error
		drivers, err = u.repository.ListReplayDriverNumbers(ctx, sessionID)
		if err != nil {
			return err
		}
	}
	sortByDriverKey(drivers)

	recorder := u.startRecording(ctx, sessionID, query)
	write := func(part domain.ReplayPart) error {
		if err := emit(part); err != nil {
			return err
		}
		recorder.save(ctx, part)
		return nil
	}

	for _, dataset := range domain.ReplayDatasets {
		if !dataset.DriverScoped {
			rows, err := u.trackRows(ctx, dataset.Name, sessionID, query.Window)
			if err != nil {
				recorder.abandon(ctx)
				return err
			}
			if err := write(domain.ReplayPart{Dataset: dataset.Name, Rows: rows}); err != nil {
				recorder.abandon(ctx)
				return err
			}
			continue
		}
		for _, driver := range drivers {
			rows, err := u.driverRows(ctx, dataset.Name, sessionID, driver, query.Window)
			if err != nil {
				recorder.abandon(ctx)
				return err
			}
			if rows == nil {
				continue
			}
			driverNumber := driver
			if err := write(domain.ReplayPart{Dataset: dataset.Name, DriverNumber: &driverNumber, Rows: rows}); err != nil {
				recorder.abandon(ctx)
				return err
			}
		}
	}
	recorder.complete(ctx, sessionID)
	return nil
}

// sortByDriverKey orders driver numbers as their string keys ("1", "11", "2"), the order
// encoding/json gave the per-driver maps of the dump.
func sortByDriverKey(drivers []int) {
	sort.Slice(drivers, func(i, j int) bool {
		return strconv.Itoa(drivers[i]) < strconv.Itoa(drivers[j])
	})
}

// driverRows returns one driver's samples of a driver-scoped dataset as a JSON array, or nil
// when the driver has none (the driver key is then left out, as before).
func (u *RaceReplayUseCase) driverRows(ctx context.Context, dataset string, sessionID string, driver int, window domain.TimeWindow) (json.RawMessage, error) {
	switch dataset {
	case "telemetry":
		frames, err := u.repository.ListTelemetryFrames(ctx, sessionID, driver, window)
		if err != nil || len(frames) == 0 {
			return nil, err
		}
		samples := make([]domain.RaceReplayTelemetrySample, 0, len(frames))
		for _, frame := range frames {
			samples = append(samples, domain.RaceReplayTelemetrySample{
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
		return json.Marshal(samples)
	case "position":
		positions, err := u.repository.ListPositionFrames(ctx, sessionID, driver, window)
		if err != nil || len(positions) == 0 {
			return nil, err
		}
		samples := make([]domain.RaceReplayPositionSample, 0, len(positions))
		for _, position := range positions {
			samples = append(samples, domain.RaceReplayPositionSample{
				Position:      position.Position,
				GapToLeader:   position.GapToLeader,
				GapAhead:      position.GapAhead,
				GapBehind:     position.GapBehind,
				LapsCompleted: position.LapsCompleted,
				Timestamp:     position.Timestamp,
			})
		}
		return json.Marshal(samples)
	case "radio":
		messages, err := u.repository.ListRadioFrames(ctx, sessionID, driver, window)
		if err != nil || len(messages) == 0 {
			return nil, err
		}
		samples := make([]domain.RaceReplayRadioSample, 0, len(messages))
		for _, message := range messages {
			samples = append(samples, domain.RaceReplayRadioSample{Timestamp: message.Timestamp, AudioURL: message.AudioURL})
		}
		return json.Marshal(samples)
	case "pitStop":
		frames, err := u.repository.ListPitStopFrames(ctx, sessionID, driver, window)
		if err != nil || len(frames) == 0 {
			return nil, err
		}
		samples := make([]domain.RaceReplayPitStopSample, 0, len(frames))
		for _, frame := range frames {
			samples = append(samples, domain.RaceReplayPitStopSample{LapNumber: frame.Payload.LapNumber, PitDuration: frame.Payload.PitDuration})
		}
		return json.Marshal(samples)
	case "stint":
		frames, err := u.repository.ListStintFrames(ctx, sessionID, driver, window)
		if err != nil || len(frames) == 0 {
			return nil, err
		}
		samples := make([]domain.RaceReplayStintSample, 0, len(frames))
		for _, frame := range frames {
			samples = append(samples, domain.RaceReplayStintSample{
				StintNumber:    frame.Payload.StintNumber,
				Compound:       frame.Payload.Compound,
				LapStart:       frame.Payload.LapStart,
				LapEnd:         frame.Payload.LapEnd,
				TyreAgeAtStart: frame.Payload.TyreAgeAtStart,
			})
		}
		return json.Marshal(samples)
	case "lap":
		frames, err := u.repository.ListLapFrames(ctx, sessionID, driver, window)
		if err != nil || len(frames) == 0 {
			return nil, err
		}
		samples := make([]domain.RaceReplayLapSample, 0, len(frames))
		for _, frame := range frames {
			samples = append(samples, domain.RaceReplayLapSample{
				LapNumber:   frame.Payload.LapNumber,
				LapDuration: frame.Payload.LapDuration,
				Sector1:     frame.Payload.Sector1,
				Sector2:     frame.Payload.Sector2,
				Sector3:     frame.Payload.Sector3,
				IsPitOutLap: frame.Payload.IsPitOutLap,
			})
		}
		return json.Marshal(samples)
	}
	return nil, nil
}

// trackRows returns a track-wide dataset as a JSON array, `[]` when empty (never null).
func (u *RaceReplayUseCase) trackRows(ctx context.Context, dataset string, sessionID string, window domain.TimeWindow) (json.RawMessage, error) {
	switch dataset {
	case "raceControl":
		events, err := u.repository.ListRaceControlFrames(ctx, sessionID, window)
		if err != nil {
			return nil, err
		}
		if events == nil {
			events = []domain.RaceControlEvent{}
		}
		return json.Marshal(events)
	case "weather":
		samples, err := u.repository.ListWeatherFrames(ctx, sessionID, window)
		if err != nil {
			return nil, err
		}
		if samples == nil {
			samples = []domain.RaceWeatherSample{}
		}
		return json.Marshal(samples)
	}
	return json.RawMessage("[]"), nil
}

// replayRecorder writes the parts of a full dump into the cache while they are streamed. Any
// cache failure only stops the recording (the response is unaffected) and drops the archive.
type replayRecorder struct {
	cache     ports.ReplayCache
	archiveID string
	next      int
}

// startRecording opens an archive for a full dump (no driver, no window); it returns a recorder
// that does nothing otherwise.
func (u *RaceReplayUseCase) startRecording(ctx context.Context, sessionID string, query domain.ReplayQuery) *replayRecorder {
	if u.cache == nil || query.DriverNumber != nil || !query.Window.IsZero() {
		return &replayRecorder{}
	}
	archiveID, err := u.cache.Create(ctx, sessionID)
	if err != nil {
		log.Printf("replay cache: create archive for session %s failed: %v", sessionID, err)
		return &replayRecorder{}
	}
	return &replayRecorder{cache: u.cache, archiveID: archiveID}
}

// save stores one part; a failure (for example the archive deleted by a concurrent ingestion)
// stops the recording.
func (r *replayRecorder) save(ctx context.Context, part domain.ReplayPart) {
	if r.cache == nil {
		return
	}
	if err := r.cache.SaveChunk(ctx, r.archiveID, r.next, part); err != nil {
		log.Printf("replay cache: save part %d of archive %s failed, recording stopped: %v", r.next, r.archiveID, err)
		r.abandon(ctx)
		return
	}
	r.next++
}

// complete makes the archive servable, or drops it when another recording of the session
// completed first.
func (r *replayRecorder) complete(ctx context.Context, sessionID string) {
	if r.cache == nil {
		return
	}
	kept, err := r.cache.MarkComplete(ctx, r.archiveID, sessionID)
	if err != nil {
		log.Printf("replay cache: complete archive %s failed: %v", r.archiveID, err)
	}
	if !kept {
		r.abandon(ctx)
	}
}

// abandon deletes the unfinished archive, even when the request context is already cancelled
// (client gone).
func (r *replayRecorder) abandon(ctx context.Context) {
	if r.cache == nil {
		return
	}
	if err := r.cache.Delete(context.WithoutCancel(ctx), r.archiveID); err != nil {
		log.Printf("replay cache: delete archive %s failed: %v", r.archiveID, err)
	}
	r.cache = nil
}
