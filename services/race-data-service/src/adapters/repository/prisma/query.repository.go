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
	"sort"
	"time"

	db "overdrive/services/race-data-service/resources/db"
	contracts "overdrive/shared/contracts/ingestion"
)

type QueryRepository struct {
	client *db.PrismaClient
}

// isNotFoundErr reports whether err is the Prisma "no row matched" sentinel — this
// client returns (nil, ErrNotFound) rather than (nil, nil) on a FindUnique/FindFirst
// miss, so callers must check for it explicitly instead of treating it as a hard error.
func isNotFoundErr(err error) bool {
	return errors.Is(err, db.ErrNotFound)
}

// NewQueryRepository builds and returns a query repository with its required dependencies.
func NewQueryRepository(client *db.PrismaClient) *QueryRepository {
	return &QueryRepository{client: client}
}

// GetSessionDataset returns the requested session dataset payload for the supplied identifiers.
func (r *QueryRepository) GetSessionDataset(ctx context.Context, sessionID string, dataset string) ([]map[string]any, error) {
	switch dataset {
	case "laps":
		rows, err := r.client.RaceLap.FindMany(db.RaceLap.SessionID.Equals(sessionID)).Exec(ctx)
		if err != nil {
			return nil, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			data = append(data, rawMap(row.Raw))
		}
		sort.Slice(data, func(i, j int) bool { return intFromAny(data[i]["lap_number"]) < intFromAny(data[j]["lap_number"]) })
		return data, nil
	case "car_data", "telemetry":
		return r.telemetryRows(ctx, sessionID, 0)
	case "location":
		return r.locationRows(ctx, sessionID, 0)
	case "position":
		return r.positionRows(ctx, sessionID, 0)
	case "intervals":
		return r.intervalRows(ctx, sessionID, 0)
	case "stints":
		return r.stintRows(ctx, sessionID, 0)
	case "pit", "pit_stops":
		return r.pitRows(ctx, sessionID, 0)
	case "weather":
		return r.weatherRows(ctx, sessionID)
	case "team_radio":
		return r.radioRows(ctx, sessionID, 0)
	case "overtakes":
		rows, err := r.client.OvertakeEvent.FindMany(db.OvertakeEvent.SessionID.Equals(sessionID)).Exec(ctx)
		if err != nil {
			return nil, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			data = append(data, rawMap(row.Raw))
		}
		return data, nil
	case "race_control":
		rows, err := r.client.RaceControlEvent.FindMany(db.RaceControlEvent.SessionID.Equals(sessionID)).Exec(ctx)
		if err != nil {
			return nil, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			data = append(data, rawMap(row.Raw))
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unknown dataset %q", dataset)
	}
}

// GetDriverDataset returns the requested driver dataset payload for the supplied identifiers.
func (r *QueryRepository) GetDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) ([]map[string]any, error) {
	switch dataset {
	case "laps":
		rows, err := r.client.RaceLap.FindMany(
			db.RaceLap.SessionID.Equals(sessionID),
			db.RaceLap.DriverNumber.Equals(driverNumber),
		).Exec(ctx)
		if err != nil {
			return nil, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			data = append(data, rawMap(row.Raw))
		}
		sort.Slice(data, func(i, j int) bool { return intFromAny(data[i]["lap_number"]) < intFromAny(data[j]["lap_number"]) })
		return data, nil
	case "telemetry":
		return r.telemetryRows(ctx, sessionID, driverNumber)
	case "location":
		return r.locationRows(ctx, sessionID, driverNumber)
	case "position":
		return r.positionRows(ctx, sessionID, driverNumber)
	case "intervals":
		return r.intervalRows(ctx, sessionID, driverNumber)
	case "stints":
		return r.stintRows(ctx, sessionID, driverNumber)
	case "pit":
		return r.pitRows(ctx, sessionID, driverNumber)
	case "radio":
		return r.radioRows(ctx, sessionID, driverNumber)
	default:
		return nil, fmt.Errorf("unknown driver dataset %q", dataset)
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

// GetDriverBroadcast returns the requested driver broadcast payload for the supplied identifiers.
func (r *QueryRepository) GetDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (string, error) {
	driverID := contracts.DriverID("openf1", "f1", fmt.Sprintf("%d", driverNumber))
	row, err := r.client.SessionDriverBroadcast.FindFirst(
		db.SessionDriverBroadcast.SessionID.Equals(sessionID),
		db.SessionDriverBroadcast.DriverID.Equals(driverID),
	).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return "", nil
		}
		return "", err
	}
	if row == nil {
		return "", nil
	}
	return row.BroadcastURL, nil
}

// telemetryRows loads telemetry rows from Prisma and converts them to response maps.
func (r *QueryRepository) telemetryRows(ctx context.Context, sessionID string, driverNumber int) ([]map[string]any, error) {
	params := []db.RaceTelemetrySampleWhereParam{db.RaceTelemetrySample.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceTelemetrySample.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceTelemetrySample.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return rawRowsTelemetry(rows), nil
}

// locationRows loads location rows from Prisma and converts them to response maps.
func (r *QueryRepository) locationRows(ctx context.Context, sessionID string, driverNumber int) ([]map[string]any, error) {
	params := []db.RaceLocationSampleWhereParam{db.RaceLocationSample.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceLocationSample.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceLocationSample.FindMany(params...).Exec(ctx)
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
func (r *QueryRepository) positionRows(ctx context.Context, sessionID string, driverNumber int) ([]map[string]any, error) {
	params := []db.RacePositionSampleWhereParam{db.RacePositionSample.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RacePositionSample.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RacePositionSample.FindMany(params...).Exec(ctx)
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
func (r *QueryRepository) intervalRows(ctx context.Context, sessionID string, driverNumber int) ([]map[string]any, error) {
	params := []db.RaceIntervalSampleWhereParam{db.RaceIntervalSample.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceIntervalSample.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceIntervalSample.FindMany(params...).Exec(ctx)
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
func (r *QueryRepository) stintRows(ctx context.Context, sessionID string, driverNumber int) ([]map[string]any, error) {
	params := []db.RaceStintWhereParam{db.RaceStint.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RaceStint.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RaceStint.FindMany(params...).Exec(ctx)
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
func (r *QueryRepository) pitRows(ctx context.Context, sessionID string, driverNumber int) ([]map[string]any, error) {
	params := []db.RacePitStopWhereParam{db.RacePitStop.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.RacePitStop.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.RacePitStop.FindMany(params...).Exec(ctx)
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
func (r *QueryRepository) weatherRows(ctx context.Context, sessionID string) ([]map[string]any, error) {
	rows, err := r.client.RaceWeatherSample.FindMany(db.RaceWeatherSample.SessionID.Equals(sessionID)).Exec(ctx)
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
func (r *QueryRepository) radioRows(ctx context.Context, sessionID string, driverNumber int) ([]map[string]any, error) {
	params := []db.TeamRadioMessageWhereParam{db.TeamRadioMessage.SessionID.Equals(sessionID)}
	if driverNumber > 0 {
		params = append(params, db.TeamRadioMessage.DriverNumber.Equals(driverNumber))
	}
	rows, err := r.client.TeamRadioMessage.FindMany(params...).Exec(ctx)
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
