/**
##
## OverDrive 2026
## All Technical rights reserved
##
## query.repository.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
##
*/

package prismaadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	db "overdrive/services/race-data-service/resources/db"
	"overdrive/services/race-data-service/src/core/domain"
)

type QueryRepository struct {
	client *db.PrismaClient
}

// isNotFoundErr reports whether err is the Prisma "no row matched" sentinel - this
// client returns (nil, ErrNotFound) rather than (nil, nil) on a FindUnique/FindFirst
// miss, so callers must check for it explicitly instead of treating it as a hard error.
func isNotFoundErr(err error) bool {
	return errors.Is(err, db.ErrNotFound)
}

// NewQueryRepository builds and returns a query repository with its required dependencies.
func NewQueryRepository(client *db.PrismaClient) *QueryRepository {
	return &QueryRepository{client: client}
}

// GetSessionDataset returns page.Fetch() rows of the requested session dataset from
// page.Offset, in a stable order (time then id for samples, lap number for laps, id otherwise).
func (r *QueryRepository) GetSessionDataset(ctx context.Context, sessionID string, dataset string, page domain.Page) ([]map[string]any, error) {
	switch dataset {
	case "laps":
		return r.lapRows(ctx, sessionID, 0, page)
	case "car_data", "telemetry":
		return r.telemetryRows(ctx, sessionID, 0, page)
	case "location":
		return r.locationRows(ctx, sessionID, 0, page)
	case "position":
		return r.positionRows(ctx, sessionID, 0, page)
	case "intervals":
		return r.intervalRows(ctx, sessionID, 0, page)
	case "stints":
		return r.stintRows(ctx, sessionID, 0, page)
	case "pit", "pit_stops":
		return r.pitRows(ctx, sessionID, 0, page)
	case "weather":
		return r.weatherRows(ctx, sessionID, page)
	case "team_radio":
		return r.radioRows(ctx, sessionID, 0, page)
	case "overtakes":
		rows, err := r.client.OvertakeEvent.FindMany(db.OvertakeEvent.SessionID.Equals(sessionID)).
			OrderBy(db.OvertakeEvent.ID.Order(db.SortOrderAsc)).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
		if err != nil {
			return nil, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			data = append(data, rawMap(row.Raw))
		}
		return data, nil
	case "race_control":
		rows, err := r.client.RaceControlEvent.FindMany(db.RaceControlEvent.SessionID.Equals(sessionID)).OrderBy(
			db.RaceControlEvent.DateUtc.Order(db.SortOrderAsc),
			db.RaceControlEvent.ID.Order(db.SortOrderAsc),
		).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
		if err != nil {
			return nil, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			data = append(data, rawMap(row.Raw))
		}
		return data, nil
	default:
		return nil, fmt.Errorf("%w %q", domain.ErrUnknownDataset, dataset)
	}
}

// GetDriverDataset returns page.Fetch() rows of the requested driver dataset from page.Offset,
// in the same stable order as GetSessionDataset.
func (r *QueryRepository) GetDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string, page domain.Page) ([]map[string]any, error) {
	switch dataset {
	case "laps":
		return r.lapRows(ctx, sessionID, driverNumber, page)
	case "telemetry":
		return r.telemetryRows(ctx, sessionID, driverNumber, page)
	case "location":
		return r.locationRows(ctx, sessionID, driverNumber, page)
	case "position":
		return r.positionRows(ctx, sessionID, driverNumber, page)
	case "intervals":
		return r.intervalRows(ctx, sessionID, driverNumber, page)
	case "stints":
		return r.stintRows(ctx, sessionID, driverNumber, page)
	case "pit":
		return r.pitRows(ctx, sessionID, driverNumber, page)
	case "radio":
		return r.radioRows(ctx, sessionID, driverNumber, page)
	default:
		return nil, fmt.Errorf("%w %q", domain.ErrUnknownDataset, dataset)
	}
}

// GetDriverLapLocation returns the requested driver lap location payload for the supplied identifiers.
func (r *QueryRepository) GetDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (map[string]any, error) {
	lap, err := r.client.RaceLap.FindUnique(
		db.RaceLap.SessionIDDriverNumberLapNumber(
			db.RaceLap.SessionID.Equals(sessionID),
			db.RaceLap.DriverNumber.Equals(driverNumber),
			db.RaceLap.LapNumber.Equals(lapNumber),
		),
	).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if lap == nil {
		return nil, nil
	}
	startedAt, ok := lap.DateStartUtc()
	if !ok {
		return map[string]any{"session_id": sessionID, "driver_number": driverNumber, "lap_number": lapNumber, "count": 0, "data": []map[string]any{}}, nil
	}
	startTime := time.Time(startedAt)
	endTime := startTime
	if lapDuration, ok := lap.LapDurationSec(); ok {
		endTime = startTime.Add(time.Duration(lapDuration * float64(time.Second)))
	} else {
		nextLap, nextErr := r.client.RaceLap.FindUnique(
			db.RaceLap.SessionIDDriverNumberLapNumber(
				db.RaceLap.SessionID.Equals(sessionID),
				db.RaceLap.DriverNumber.Equals(driverNumber),
				db.RaceLap.LapNumber.Equals(lapNumber+1),
			),
		).Exec(ctx)
		if nextErr == nil && nextLap != nil {
			if nextStart, nextOK := nextLap.DateStartUtc(); nextOK {
				endTime = time.Time(nextStart)
			}
		}
	}
	samples, err := r.client.RaceLocationSample.FindMany(
		db.RaceLocationSample.SessionID.Equals(sessionID),
		db.RaceLocationSample.DriverNumber.Equals(driverNumber),
		db.RaceLocationSample.DateUtc.Gte(db.DateTime(startTime)),
		db.RaceLocationSample.DateUtc.Lt(db.DateTime(endTime)),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(samples))
	for _, sample := range samples {
		item := rawMap(sample.Raw)
		data = append(data, item)
	}
	return map[string]any{
		"session_id":    sessionID,
		"driver_number": driverNumber,
		"lap_number":    lapNumber,
		"window_start":  startTime.UTC().Format(time.RFC3339),
		"window_end":    endTime.UTC().Format(time.RFC3339),
		"count":         len(data),
		"data":          data,
	}, nil
}

// GetDriverBroadcast returns the driver's stored feed list (empty when no row exists).
func (r *QueryRepository) GetDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) ([]domain.Feed, error) {
	return loadDriverFeeds(ctx, r.client, sessionID, driverNumber)
}

// lapRows loads lap rows ordered by lap number (then driver), optionally for one driver.
func (r *QueryRepository) lapRows(ctx context.Context, sessionID string, driverNumber int, page domain.Page) ([]map[string]any, error) {
	params := []db.RaceLapWhereParam{db.RaceLap.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceLap.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceLap.FindMany(params...).OrderBy(
		db.RaceLap.LapNumber.Order(db.SortOrderAsc),
		db.RaceLap.DriverNumber.Order(db.SortOrderAsc),
	).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data, nil
}

// telemetryRows loads telemetry rows from Prisma and converts them to response maps.
func (r *QueryRepository) telemetryRows(ctx context.Context, sessionID string, driverNumber int, page domain.Page) ([]map[string]any, error) {
	params := []db.RaceTelemetrySampleWhereParam{db.RaceTelemetrySample.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceTelemetrySample.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceTelemetrySample.FindMany(params...).OrderBy(
		db.RaceTelemetrySample.DateUtc.Order(db.SortOrderAsc),
		db.RaceTelemetrySample.ID.Order(db.SortOrderAsc),
	).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return rawRowsTelemetry(rows), nil
}

// locationRows loads location rows from Prisma and converts them to response maps.
func (r *QueryRepository) locationRows(ctx context.Context, sessionID string, driverNumber int, page domain.Page) ([]map[string]any, error) {
	params := []db.RaceLocationSampleWhereParam{db.RaceLocationSample.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceLocationSample.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceLocationSample.FindMany(params...).OrderBy(
		db.RaceLocationSample.DateUtc.Order(db.SortOrderAsc),
		db.RaceLocationSample.ID.Order(db.SortOrderAsc),
	).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data, nil
}

// positionRows loads position rows from Prisma and converts them to response maps.
func (r *QueryRepository) positionRows(ctx context.Context, sessionID string, driverNumber int, page domain.Page) ([]map[string]any, error) {
	params := []db.RacePositionSampleWhereParam{db.RacePositionSample.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RacePositionSample.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RacePositionSample.FindMany(params...).OrderBy(
		db.RacePositionSample.DateUtc.Order(db.SortOrderAsc),
		db.RacePositionSample.ID.Order(db.SortOrderAsc),
	).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data, nil
}

// intervalRows loads interval rows from Prisma and converts them to response maps.
func (r *QueryRepository) intervalRows(ctx context.Context, sessionID string, driverNumber int, page domain.Page) ([]map[string]any, error) {
	params := []db.RaceIntervalSampleWhereParam{db.RaceIntervalSample.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceIntervalSample.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceIntervalSample.FindMany(params...).OrderBy(
		db.RaceIntervalSample.DateUtc.Order(db.SortOrderAsc),
		db.RaceIntervalSample.ID.Order(db.SortOrderAsc),
	).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data, nil
}

// stintRows loads stint rows from Prisma and converts them to response maps.
func (r *QueryRepository) stintRows(ctx context.Context, sessionID string, driverNumber int, page domain.Page) ([]map[string]any, error) {
	params := []db.RaceStintWhereParam{db.RaceStint.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceStint.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceStint.FindMany(params...).OrderBy(db.RaceStint.ID.Order(db.SortOrderAsc)).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data, nil
}

// pitRows loads pit rows from Prisma and converts them to response maps.
func (r *QueryRepository) pitRows(ctx context.Context, sessionID string, driverNumber int, page domain.Page) ([]map[string]any, error) {
	params := []db.RacePitStopWhereParam{db.RacePitStop.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RacePitStop.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RacePitStop.FindMany(params...).OrderBy(db.RacePitStop.ID.Order(db.SortOrderAsc)).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data, nil
}

// weatherRows loads weather rows from Prisma and converts them to response maps.
func (r *QueryRepository) weatherRows(ctx context.Context, sessionID string, page domain.Page) ([]map[string]any, error) {
	rows, err := r.client.RaceWeatherSample.FindMany(db.RaceWeatherSample.SessionID.Equals(sessionID)).OrderBy(
		db.RaceWeatherSample.DateUtc.Order(db.SortOrderAsc),
		db.RaceWeatherSample.ID.Order(db.SortOrderAsc),
	).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data, nil
}

// radioRows loads radio rows from Prisma and converts them to response maps.
func (r *QueryRepository) radioRows(ctx context.Context, sessionID string, driverNumber int, page domain.Page) ([]map[string]any, error) {
	params := []db.TeamRadioMessageWhereParam{db.TeamRadioMessage.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.TeamRadioMessage.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.TeamRadioMessage.FindMany(params...).OrderBy(db.TeamRadioMessage.ID.Order(db.SortOrderAsc)).Skip(page.Offset).Take(page.Fetch()).Exec(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data, nil
}

// rawRowsTelemetry converts telemetry Prisma rows into raw response maps.
func rawRowsTelemetry(rows []db.RaceTelemetrySampleModel) []map[string]any {
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, rawMap(row.Raw))
	}
	return data
}

// rawMap unwraps Prisma JSON into a mutable map response payload.
func rawMap(value db.JSON) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

// intFromAny converts common numeric payload values into an int.
func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
