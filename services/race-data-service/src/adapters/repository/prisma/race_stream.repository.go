/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_stream.repository.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
	##
*/

package prismaadapter

import (
	"context"
	"sort"
	"strings"
	"time"

	db "overdrive/services/race-data-service/resources/db"
	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// RaceStreamRepository reads session-scoped historical samples, oldest first,
// backing the GET /sessions/{sessionId}/race/replay bulk dump.
type RaceStreamRepository struct {
	client *db.PrismaClient
}

// NewRaceStreamRepository builds and returns a race stream repository with its required dependencies.
func NewRaceStreamRepository(client *db.PrismaClient) *RaceStreamRepository {
	return &RaceStreamRepository{client: client}
}

// ListTelemetryFrames returns telemetry samples oldest-first, optionally
// scoped to a single driver.
//
// x/y/z and lapNumber are not columns on RaceTelemetrySample — x/y/z live on
// RaceLocationSample (sampled independently, its own timestamps) and
// lapNumber is derived from RaceLap's recorded start times (OpenF1's
// `position` resource has no lap_number field at all, so RacePositionSample
// can't be used for this — verified against
// ingestion-service/src/adapters/providers/openf1/mapper.go's mapPosition).
// Rather than a per-row lookup (one extra DB round trip per telemetry
// sample), both tables are fetched once per call and matched via a
// forward-only pointer walk against the already-ascending-sorted results.
func (r *RaceStreamRepository) ListTelemetryFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceReplayTelemetryFrame, error) {
	telemetryParams := []db.RaceTelemetrySampleWhereParam{db.RaceTelemetrySample.SessionID.Equals(sessionID)}
	locationParams := []db.RaceLocationSampleWhereParam{db.RaceLocationSample.SessionID.Equals(sessionID)}
	lapParams := []db.RaceLapWhereParam{db.RaceLap.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		telemetryParams = append(telemetryParams, db.RaceTelemetrySample.DriverNumber.Equals(*driverNumber))
		locationParams = append(locationParams, db.RaceLocationSample.DriverNumber.Equals(*driverNumber))
		lapParams = append(lapParams, db.RaceLap.DriverNumber.Equals(*driverNumber))
	}

	rows, err := r.client.RaceTelemetrySample.FindMany(telemetryParams...).OrderBy(db.RaceTelemetrySample.DateUtc.Order(db.SortOrderAsc)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	locationRows, err := r.client.RaceLocationSample.FindMany(locationParams...).OrderBy(db.RaceLocationSample.DateUtc.Order(db.SortOrderAsc)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	lapRows, err := r.client.RaceLap.FindMany(lapParams...).OrderBy(db.RaceLap.LapNumber.Order(db.SortOrderAsc)).Exec(ctx)
	if err != nil {
		return nil, err
	}

	locationIdx, lapIdx := 0, 0
	items := make([]domain.RaceReplayTelemetryFrame, 0, len(rows))
	for _, row := range rows {
		speed, _ := row.SpeedKph()
		rpm, _ := row.Rpm()
		gear, _ := row.Gear()
		throttle, _ := row.ThrottlePct()
		brake, _ := row.BrakePct()
		drsState, _ := row.DrsState()
		sampleTime := time.Time(row.DateUtc)

		x, y, z := nearestLocation(locationRows, &locationIdx, sampleTime)
		lapNumber := currentLapNumber(lapRows, &lapIdx, sampleTime)

		items = append(items, domain.RaceReplayTelemetryFrame{
			DriverNumber:    row.DriverNumber,
			Speed:           speed,
			Rpm:             rpm,
			Gear:            gear,
			ThrottlePercent: throttle,
			BrakePercent:    brake,
			LapNumber:       lapNumber,
			X:               x,
			Y:               y,
			Z:               z,
			DrsActive:       drsState >= 10,
			Timestamp:       sampleTime,
		})
	}
	return items, nil
}

// nearestLocation advances idx through locationRows (sorted ascending by
// DateUtc) to the sample whose timestamp is closest to t, then returns its
// x/y/z. idx is only ever moved forward, never reset — see ListTelemetryFrames.
func nearestLocation(rows []db.RaceLocationSampleModel, idx *int, t time.Time) (float64, float64, float64) {
	if len(rows) == 0 {
		return 0, 0, 0
	}
	for *idx < len(rows)-1 && closerTo(t, rows[*idx+1].DateUtc, rows[*idx].DateUtc) {
		*idx++
	}
	x, _ := rows[*idx].X()
	y, _ := rows[*idx].Y()
	z, _ := rows[*idx].Z()
	return x, y, z
}

// currentLapNumber advances idx through lapRows (sorted ascending by
// lapNumber) to the last lap whose recorded start time is at or before t —
// i.e. the lap actually in progress at t. Unlike location, lap number is a
// monotonic step function, not a continuous signal: "nearest by absolute
// distance" would occasionally jump to the next lap early (right before its
// start); floor semantics ("last lap that had already started") are what we
// actually want. Laps with no recorded dateStartUtc (nullable in the schema)
// are never used as the next boundary, so a gap in that data just keeps the
// previous known lap number instead of guessing.
func currentLapNumber(rows []db.RaceLapModel, idx *int, t time.Time) int {
	if len(rows) == 0 {
		return 0
	}
	for *idx < len(rows)-1 {
		nextStart, ok := rows[*idx+1].DateStartUtc()
		if !ok || time.Time(nextStart).After(t) {
			break
		}
		*idx++
	}
	return rows[*idx].LapNumber
}

// closerTo reports whether candidate is at least as close to t as current is.
func closerTo(t time.Time, candidate db.DateTime, current db.DateTime) bool {
	return absDuration(time.Time(candidate).Sub(t)) <= absDuration(time.Time(current).Sub(t))
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// ListPositionFrames returns position samples oldest-first, optionally scoped
// to a single driver. Unlike RaceLiveRepository.ListPositions (which returns
// only the LATEST sample per driver, enriched with gapToLeader/gapAhead from
// the intervals dataset), this returns every historical sample for replay and
// intentionally leaves the gap fields empty — recomputing gaps per tick would
// require an extra interval lookup per sample, which isn't worth it for a V1
// replay. Use GET /sessions/{sessionId}/race/position for gap-enriched data.
func (r *RaceStreamRepository) ListPositionFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePosition, error) {
	params := []db.RacePositionSampleWhereParam{db.RacePositionSample.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.RacePositionSample.DriverNumber.Equals(*driverNumber))
	}
	rows, err := r.client.RacePositionSample.FindMany(params...).OrderBy(db.RacePositionSample.DateUtc.Order(db.SortOrderAsc)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RacePosition, 0, len(rows))
	for _, row := range rows {
		position, _ := row.Position()
		lapsCompleted, _ := row.LapNumber()
		items = append(items, domain.RacePosition{
			DriverNumber:  row.DriverNumber,
			Position:      position,
			LapsCompleted: lapsCompleted,
			Timestamp:     time.Time(row.DateUtc),
		})
	}
	return items, nil
}

// ListRaceControlFrames returns race control events oldest-first for the
// session. Not filtered by driver: flags/safety car/track status apply
// track-wide, so they are always included in the stream regardless of the
// `driverNumber` query param.
func (r *RaceStreamRepository) ListRaceControlFrames(ctx context.Context, sessionID string) ([]domain.RaceControlEvent, error) {
	rows, err := r.client.RaceControlEvent.FindMany(
		db.RaceControlEvent.SessionID.Equals(sessionID),
	).OrderBy(db.RaceControlEvent.DateUtc.Order(db.SortOrderAsc)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RaceControlEvent, 0, len(rows))
	for _, row := range rows {
		category, _ := row.Category()
		flag, _ := row.Flag()
		scope, _ := row.Scope()
		message, _ := row.Message()
		lapNumber, _ := row.LapNumber()
		event := domain.RaceControlEvent{
			Category:  category,
			Flag:      flag,
			Message:   message,
			LapNumber: lapNumber,
			Timestamp: time.Time(row.DateUtc),
			SafetyCar: safetyCarFromScope(scope, category),
		}
		items = append(items, event)
	}
	return items, nil
}

// safetyCarFromScope mirrors usecases.safetyCarFromScope (unexported in that
// package, so duplicated here): a best-effort heuristic deriving the public
// safetyCar contract value ("SC"/"VSC"/nil) from OpenF1's scope/category
// fields, since there is no dedicated safety-car flag upstream.
func safetyCarFromScope(scope string, category string) *string {
	combined := strings.ToLower(scope + " " + category)
	switch {
	case strings.Contains(combined, "virtual"):
		value := "VSC"
		return &value
	case strings.Contains(combined, "safety car") || strings.Contains(combined, "safetycar"):
		value := "SC"
		return &value
	default:
		return nil
	}
}

// ListWeatherFrames returns all weather samples for a session, oldest first.
// Never filtered by driver — weather is track-wide, matching
// GET /race/weather not taking a driverNumber param either.
func (r *RaceStreamRepository) ListWeatherFrames(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error) {
	rows, err := r.client.RaceWeatherSample.FindMany(
		db.RaceWeatherSample.SessionID.Equals(sessionID),
	).OrderBy(db.RaceWeatherSample.DateUtc.Order(db.SortOrderAsc)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RaceWeatherSample, 0, len(rows))
	for _, row := range rows {
		item := domain.RaceWeatherSample{Timestamp: time.Time(row.DateUtc)}
		if v, ok := row.AirTempC(); ok {
			item.AirTemperature = &v
		}
		if v, ok := row.TrackTempC(); ok {
			item.TrackTemperature = &v
		}
		if v, ok := row.HumidityPct(); ok {
			item.Humidity = &v
		}
		if v, ok := row.WindSpeedKph(); ok {
			item.WindSpeed = &v
		}
		if v, ok := row.RainfallMm(); ok {
			item.Rainfall = v > 0
		}
		items = append(items, item)
	}
	return items, nil
}

// ListRadioFrames returns team radio messages oldest-first, optionally
// scoped to a single driver. dateUtc is nullable on TeamRadioMessage — a
// message with no recorded timestamp cannot be placed on the replay
// timeline, so it is skipped rather than guessed at.
func (r *RaceStreamRepository) ListRadioFrames(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error) {
	params := []db.TeamRadioMessageWhereParam{db.TeamRadioMessage.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.TeamRadioMessage.DriverNumber.Equals(*driverNumber))
	}
	rows, err := r.client.TeamRadioMessage.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RaceRadioMessage, 0, len(rows))
	for _, row := range rows {
		dateUtc, ok := row.DateUtc()
		if !ok {
			continue
		}
		item := domain.RaceRadioMessage{DriverNumber: row.DriverNumber, Timestamp: time.Time(dateUtc)}
		if url, ok := row.RecordingURL(); ok {
			item.AudioURL = url
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp.Before(items[j].Timestamp) })
	return items, nil
}

// ListPitStopFrames returns pit stops oldest-first, optionally scoped to a
// single driver. dateUtc is nullable on RacePitStop — a pit stop with no
// recorded timestamp cannot be placed on the replay timeline, so it is
// skipped rather than guessed at.
func (r *RaceStreamRepository) ListPitStopFrames(ctx context.Context, sessionID string, driverNumber *int) ([]ports.RaceReplayPitStopFrame, error) {
	params := []db.RacePitStopWhereParam{db.RacePitStop.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.RacePitStop.DriverNumber.Equals(*driverNumber))
	}
	rows, err := r.client.RacePitStop.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]ports.RaceReplayPitStopFrame, 0, len(rows))
	for _, row := range rows {
		dateUtc, ok := row.DateUtc()
		if !ok {
			continue
		}
		lapNumber, _ := row.LapNumber()
		duration, _ := row.PitDurationSec()
		items = append(items, ports.RaceReplayPitStopFrame{
			Payload: domain.RacePitStop{
				DriverNumber: row.DriverNumber,
				LapNumber:    lapNumber,
				PitDuration:  duration,
			},
			Timestamp: time.Time(dateUtc),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp.Before(items[j].Timestamp) })
	return items, nil
}

// ListStintFrames returns stints, optionally scoped to a single driver, each
// paired with an APPROXIMATED timestamp anchored to the start of its
// lapStart lap. RaceStint has no timestamp column at all in the schema (only
// stintNumber/lapStart/lapEnd/compound/tyreAgeLapsStart) — see
// ports.RaceReplayStintFrame for why this approximation was chosen. Stints
// missing lapStart, or whose lapStart lap has no recorded dateStartUtc, are
// skipped from the stream entirely rather than placed with a guessed time.
//
// The lap-start lookup is built once per call (all of the session's/driver's
// laps fetched in a single query and indexed by driverNumber+lapNumber) so
// this stays O(1) DB round trips regardless of stint count, following the
// same "fetch once, index in memory" approach as nearestLocation/
// currentLapNumber above for telemetry.
func (r *RaceStreamRepository) ListStintFrames(ctx context.Context, sessionID string, driverNumber *int) ([]ports.RaceReplayStintFrame, error) {
	stintParams := []db.RaceStintWhereParam{db.RaceStint.SessionID.Equals(sessionID)}
	lapParams := []db.RaceLapWhereParam{db.RaceLap.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		stintParams = append(stintParams, db.RaceStint.DriverNumber.Equals(*driverNumber))
		lapParams = append(lapParams, db.RaceLap.DriverNumber.Equals(*driverNumber))
	}

	stintRows, err := r.client.RaceStint.FindMany(stintParams...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	lapRows, err := r.client.RaceLap.FindMany(lapParams...).Exec(ctx)
	if err != nil {
		return nil, err
	}

	lapStartByDriverAndNumber := make(map[int]map[int]time.Time, len(lapRows))
	for _, lap := range lapRows {
		startedAt, ok := lap.DateStartUtc()
		if !ok {
			continue
		}
		byNumber, exists := lapStartByDriverAndNumber[lap.DriverNumber]
		if !exists {
			byNumber = make(map[int]time.Time)
			lapStartByDriverAndNumber[lap.DriverNumber] = byNumber
		}
		byNumber[lap.LapNumber] = time.Time(startedAt)
	}

	items := make([]ports.RaceReplayStintFrame, 0, len(stintRows))
	for _, row := range stintRows {
		lapStart, ok := row.LapStart()
		if !ok {
			continue
		}
		byNumber, exists := lapStartByDriverAndNumber[row.DriverNumber]
		if !exists {
			continue
		}
		anchor, exists := byNumber[lapStart]
		if !exists {
			continue
		}
		stintNumber, _ := row.StintNumber()
		lapEnd, _ := row.LapEnd()
		compound, _ := row.Compound()
		tyreAge, _ := row.TyreAgeLapsStart()
		items = append(items, ports.RaceReplayStintFrame{
			Payload: domain.RaceStint{
				DriverNumber:   row.DriverNumber,
				StintNumber:    stintNumber,
				Compound:       compound,
				LapStart:       lapStart,
				LapEnd:         lapEnd,
				TyreAgeAtStart: tyreAge,
			},
			Timestamp: anchor,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp.Before(items[j].Timestamp) })
	return items, nil
}

// ListLapFrames returns completed laps, optionally scoped to a single
// driver, each paired with its completion time (dateStartUtc +
// lapDurationSec). Lap timing (duration/sectors) is only known once a lap
// has actually finished, so the event fires then rather than at lap start.
// Laps missing dateStartUtc or lapDurationSec (both nullable in the schema)
// are skipped rather than emitted with a guessed or zero timestamp.
func (r *RaceStreamRepository) ListLapFrames(ctx context.Context, sessionID string, driverNumber *int) ([]ports.RaceReplayLapFrame, error) {
	params := []db.RaceLapWhereParam{db.RaceLap.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.RaceLap.DriverNumber.Equals(*driverNumber))
	}
	rows, err := r.client.RaceLap.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]ports.RaceReplayLapFrame, 0, len(rows))
	for _, row := range rows {
		startedAt, hasStart := row.DateStartUtc()
		duration, hasDuration := row.LapDurationSec()
		if !hasStart || !hasDuration {
			continue
		}
		completedAt := time.Time(startedAt).Add(time.Duration(duration * float64(time.Second)))
		payload := domain.RaceReplayLapEvent{
			DriverNumber: row.DriverNumber,
			LapNumber:    row.LapNumber,
			LapDuration:  &duration,
		}
		if s1, ok := row.Sector1Sec(); ok {
			payload.Sector1 = &s1
		}
		if s2, ok := row.Sector2Sec(); ok {
			payload.Sector2 = &s2
		}
		if s3, ok := row.Sector3Sec(); ok {
			payload.Sector3 = &s3
		}
		if pitOut, ok := row.IsPitOutLap(); ok {
			payload.IsPitOutLap = pitOut
		}
		items = append(items, ports.RaceReplayLapFrame{Payload: payload, Timestamp: completedAt})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp.Before(items[j].Timestamp) })
	return items, nil
}
