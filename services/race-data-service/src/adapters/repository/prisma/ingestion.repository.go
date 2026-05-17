/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.repository.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
	##
*/

package prismaadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	db "overdrive/services/race-data-service/resources/db"
	contracts "overdrive/shared/contracts/ingestion"
)

type IngestionRepository struct {
	client *db.PrismaClient
}

// NewIngestionRepository builds and returns a ingestion repository with its required dependencies.
func NewIngestionRepository(client *db.PrismaClient) *IngestionRepository {
	return &IngestionRepository{client: client}
}

// StoreBatch persists the provided ingestion batch into the service database.
func (r *IngestionRepository) StoreBatch(ctx context.Context, batch contracts.Batch) error {
	sessionID := contracts.SessionID(batch.Provider, batch.Context.SessionKey)

	for _, dataset := range batch.Datasets {
		switch dataset.Name {
		case "lap_timing":
			if err := r.storeLaps(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store lap_timing: %w", err)
			}
		case "telemetry":
			if err := r.storeTelemetry(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store telemetry: %w", err)
			}
		case "location":
			if err := r.storeLocation(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store location: %w", err)
			}
		case "position":
			if err := r.storePosition(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store position: %w", err)
			}
		case "intervals":
			if err := r.storeIntervals(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store intervals: %w", err)
			}
		case "stints":
			if err := r.storeStints(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store stints: %w", err)
			}
		case "pit_stops":
			if err := r.storePitStops(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store pit_stops: %w", err)
			}
		case "weather":
			if err := r.storeWeather(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store weather: %w", err)
			}
		case "team_radio":
			if err := r.storeTeamRadio(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store team_radio: %w", err)
			}
		case "overtakes":
			if err := r.storeOvertakes(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store overtakes: %w", err)
			}
		case "race_control":
			if err := r.storeRaceControl(ctx, sessionID, dataset.Rows); err != nil {
				return fmt.Errorf("store race_control: %w", err)
			}
		default:
			return fmt.Errorf("unsupported race dataset %q", dataset.Name)
		}
		if err := r.ensureDriverBroadcasts(ctx, sessionID, batch.Provider, batch.Context.SessionKey, dataset.Rows); err != nil {
			return fmt.Errorf("ensure driver broadcasts: %w", err)
		}
	}

	return nil
}

// ensureDriverBroadcasts finds or creates the related database record needed by ingestion.
func (r *IngestionRepository) ensureDriverBroadcasts(ctx context.Context, sessionID string, provider string, sessionKey int, rows []map[string]any) error {
	seen := map[int]struct{}{}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		if _, ok := seen[driverNumber]; ok {
			continue
		}
		seen[driverNumber] = struct{}{}
		driverID := contracts.DriverID(provider, "f1", fmt.Sprintf("%d", driverNumber))
		url := fmt.Sprintf("https://www.youtube.com/watch?v=dQw4w9WgXcQ&session=%d&driver=%d", sessionKey, driverNumber)
		_, err := r.client.SessionDriverBroadcast.UpsertOne(
			db.SessionDriverBroadcast.SessionIDDriverID(
				db.SessionDriverBroadcast.SessionID.Equals(sessionID),
				db.SessionDriverBroadcast.DriverID.Equals(driverID),
			),
		).CreateOrUpdate(
			db.SessionDriverBroadcast.SessionID.Set(sessionID),
			db.SessionDriverBroadcast.DriverID.Set(driverID),
			db.SessionDriverBroadcast.BroadcastURL.Set(url),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeLaps persists mapped laps rows into the service database.
func (r *IngestionRepository) storeLaps(ctx context.Context, sessionID string, rows []map[string]any) error {
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		lapNumber := intValue(row["lap_number"])
		if driverNumber == 0 || lapNumber == 0 {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		dateStart := parseOptionalTime(row["started_at_utc"])
		_, err = r.client.RaceLap.UpsertOne(
			db.RaceLap.SessionIDDriverNumberLapNumber(
				db.RaceLap.SessionID.Equals(sessionID),
				db.RaceLap.DriverNumber.Equals(driverNumber),
				db.RaceLap.LapNumber.Equals(lapNumber),
			),
		).CreateOrUpdate(
			db.RaceLap.SessionID.Set(sessionID),
			db.RaceLap.DriverNumber.Set(driverNumber),
			db.RaceLap.LapNumber.Set(lapNumber),
			db.RaceLap.Raw.Set(payload),
			db.RaceLap.DateStartUtc.SetIfPresent(dateStart),
			db.RaceLap.LapDurationSec.SetIfPresent(floatPtr(row["lap_duration_sec"])),
			db.RaceLap.Sector1Sec.SetIfPresent(floatPtr(row["sector_1_sec"])),
			db.RaceLap.Sector2Sec.SetIfPresent(floatPtr(row["sector_2_sec"])),
			db.RaceLap.Sector3Sec.SetIfPresent(floatPtr(row["sector_3_sec"])),
			db.RaceLap.SpeedI1Kph.SetIfPresent(intPtr(row["speed_i1_kph"])),
			db.RaceLap.SpeedI2Kph.SetIfPresent(intPtr(row["speed_i2_kph"])),
			db.RaceLap.SpeedTrapKph.SetIfPresent(intPtr(row["speed_trap_kph"])),
			db.RaceLap.IsPitOutLap.SetIfPresent(boolPtr(row["is_pit_out_lap"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeTelemetry persists mapped telemetry rows into the service database.
func (r *IngestionRepository) storeTelemetry(ctx context.Context, sessionID string, rows []map[string]any) error {
	if err := r.deleteTelemetryScope(ctx, sessionID, rows); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		dateUtc, ok := parseTime(row["sample_time_utc"])
		if driverNumber == 0 || !ok {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.RaceTelemetrySample.CreateOne(
			db.RaceTelemetrySample.SessionID.Set(sessionID),
			db.RaceTelemetrySample.DriverNumber.Set(driverNumber),
			db.RaceTelemetrySample.DateUtc.Set(dateUtc),
			db.RaceTelemetrySample.Raw.Set(payload),
			db.RaceTelemetrySample.SpeedKph.SetIfPresent(intPtr(row["speed_kph"])),
			db.RaceTelemetrySample.Rpm.SetIfPresent(intPtr(row["rpm"])),
			db.RaceTelemetrySample.Gear.SetIfPresent(intPtr(row["gear"])),
			db.RaceTelemetrySample.ThrottlePct.SetIfPresent(floatPtr(row["throttle_pct"])),
			db.RaceTelemetrySample.BrakePct.SetIfPresent(floatPtr(row["brake_pct"])),
			db.RaceTelemetrySample.DrsState.SetIfPresent(intPtr(row["drs_state"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeLocation persists mapped location rows into the service database.
func (r *IngestionRepository) storeLocation(ctx context.Context, sessionID string, rows []map[string]any) error {
	if err := r.deleteLocationScope(ctx, sessionID, rows); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		dateUtc, ok := parseTime(row["sample_time_utc"])
		if driverNumber == 0 || !ok {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.RaceLocationSample.CreateOne(
			db.RaceLocationSample.SessionID.Set(sessionID),
			db.RaceLocationSample.DriverNumber.Set(driverNumber),
			db.RaceLocationSample.DateUtc.Set(dateUtc),
			db.RaceLocationSample.Raw.Set(payload),
			db.RaceLocationSample.X.SetIfPresent(floatPtr(row["x"])),
			db.RaceLocationSample.Y.SetIfPresent(floatPtr(row["y"])),
			db.RaceLocationSample.Z.SetIfPresent(floatPtr(row["z"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storePosition persists mapped position rows into the service database.
func (r *IngestionRepository) storePosition(ctx context.Context, sessionID string, rows []map[string]any) error {
	if err := r.deletePositionScope(ctx, sessionID, rows); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		dateUtc, ok := parseTime(row["sample_time_utc"])
		if driverNumber == 0 || !ok {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.RacePositionSample.CreateOne(
			db.RacePositionSample.SessionID.Set(sessionID),
			db.RacePositionSample.DriverNumber.Set(driverNumber),
			db.RacePositionSample.DateUtc.Set(dateUtc),
			db.RacePositionSample.Raw.Set(payload),
			db.RacePositionSample.LapNumber.SetIfPresent(intPtr(row["lap_number"])),
			db.RacePositionSample.Position.SetIfPresent(intPtr(row["position"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeIntervals persists mapped intervals rows into the service database.
func (r *IngestionRepository) storeIntervals(ctx context.Context, sessionID string, rows []map[string]any) error {
	if err := r.deleteIntervalsScope(ctx, sessionID, rows); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		dateUtc, ok := parseTime(row["sample_time_utc"])
		if driverNumber == 0 || !ok {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.RaceIntervalSample.CreateOne(
			db.RaceIntervalSample.SessionID.Set(sessionID),
			db.RaceIntervalSample.DriverNumber.Set(driverNumber),
			db.RaceIntervalSample.DateUtc.Set(dateUtc),
			db.RaceIntervalSample.Raw.Set(payload),
			db.RaceIntervalSample.GapToLeader.SetIfPresent(stringPtr(row["gap_to_leader"])),
			db.RaceIntervalSample.IntervalToFrontDriver.SetIfPresent(stringPtr(row["interval_to_front_driver"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeStints persists mapped stints rows into the service database.
func (r *IngestionRepository) storeStints(ctx context.Context, sessionID string, rows []map[string]any) error {
	if err := r.deleteStintsScope(ctx, sessionID, rows); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.RaceStint.CreateOne(
			db.RaceStint.SessionID.Set(sessionID),
			db.RaceStint.DriverNumber.Set(driverNumber),
			db.RaceStint.Raw.Set(payload),
			db.RaceStint.StintNumber.SetIfPresent(intPtr(row["stint_number"])),
			db.RaceStint.LapStart.SetIfPresent(intPtr(row["lap_start"])),
			db.RaceStint.LapEnd.SetIfPresent(intPtr(row["lap_end"])),
			db.RaceStint.Compound.SetIfPresent(stringPtr(row["compound"])),
			db.RaceStint.TyreAgeLapsStart.SetIfPresent(intPtr(row["tyre_age_laps_start"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storePitStops persists mapped pit stops rows into the service database.
func (r *IngestionRepository) storePitStops(ctx context.Context, sessionID string, rows []map[string]any) error {
	if err := r.deletePitScope(ctx, sessionID, rows); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.RacePitStop.CreateOne(
			db.RacePitStop.SessionID.Set(sessionID),
			db.RacePitStop.DriverNumber.Set(driverNumber),
			db.RacePitStop.Raw.Set(payload),
			db.RacePitStop.LapNumber.SetIfPresent(intPtr(row["lap_number"])),
			db.RacePitStop.DateUtc.SetIfPresent(parseOptionalTime(row["date_utc"])),
			db.RacePitStop.PitDurationSec.SetIfPresent(floatPtr(row["pit_duration_sec"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeWeather persists mapped weather rows into the service database.
func (r *IngestionRepository) storeWeather(ctx context.Context, sessionID string, rows []map[string]any) error {
	if _, err := r.client.RaceWeatherSample.FindMany(db.RaceWeatherSample.SessionID.Equals(sessionID)).Delete().Exec(ctx); err != nil {
		return err
	}
	for _, row := range rows {
		dateUtc, ok := parseTime(row["sample_time_utc"])
		if !ok {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.RaceWeatherSample.CreateOne(
			db.RaceWeatherSample.SessionID.Set(sessionID),
			db.RaceWeatherSample.DateUtc.Set(dateUtc),
			db.RaceWeatherSample.Raw.Set(payload),
			db.RaceWeatherSample.AirTempC.SetIfPresent(floatPtr(row["air_temp_c"])),
			db.RaceWeatherSample.TrackTempC.SetIfPresent(floatPtr(row["track_temp_c"])),
			db.RaceWeatherSample.HumidityPct.SetIfPresent(floatPtr(row["humidity_pct"])),
			db.RaceWeatherSample.PressureHpa.SetIfPresent(floatPtr(row["pressure_hpa"])),
			db.RaceWeatherSample.WindSpeedKph.SetIfPresent(floatPtr(row["wind_speed_kph"])),
			db.RaceWeatherSample.WindDirectionDeg.SetIfPresent(intPtr(row["wind_direction_deg"])),
			db.RaceWeatherSample.RainfallMm.SetIfPresent(floatPtr(row["rainfall_mm"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeTeamRadio persists mapped team radio rows into the service database.
func (r *IngestionRepository) storeTeamRadio(ctx context.Context, sessionID string, rows []map[string]any) error {
	if err := r.deleteRadioScope(ctx, sessionID, rows); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.TeamRadioMessage.CreateOne(
			db.TeamRadioMessage.SessionID.Set(sessionID),
			db.TeamRadioMessage.DriverNumber.Set(driverNumber),
			db.TeamRadioMessage.Raw.Set(payload),
			db.TeamRadioMessage.DateUtc.SetIfPresent(parseOptionalTime(row["date_utc"])),
			db.TeamRadioMessage.RecordingURL.SetIfPresent(stringPtr(row["recording_url"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeOvertakes persists mapped overtakes rows into the service database.
func (r *IngestionRepository) storeOvertakes(ctx context.Context, sessionID string, rows []map[string]any) error {
	if _, err := r.client.OvertakeEvent.FindMany(db.OvertakeEvent.SessionID.Equals(sessionID)).Delete().Exec(ctx); err != nil {
		return err
	}
	for _, row := range rows {
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.OvertakeEvent.CreateOne(
			db.OvertakeEvent.SessionID.Set(sessionID),
			db.OvertakeEvent.Raw.Set(payload),
			db.OvertakeEvent.DateUtc.SetIfPresent(parseOptionalTime(row["date_utc"])),
			db.OvertakeEvent.LapNumber.SetIfPresent(intPtr(row["lap_number"])),
			db.OvertakeEvent.OvertakingDriverNumber.SetIfPresent(intPtr(row["overtaking_driver_number"])),
			db.OvertakeEvent.OvertakenDriverNumber.SetIfPresent(intPtr(row["overtaken_driver_number"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeRaceControl persists mapped race control rows into the service database.
func (r *IngestionRepository) storeRaceControl(ctx context.Context, sessionID string, rows []map[string]any) error {
	if _, err := r.client.RaceControlEvent.FindMany(db.RaceControlEvent.SessionID.Equals(sessionID)).Delete().Exec(ctx); err != nil {
		return err
	}
	for _, row := range rows {
		dateUtc, ok := parseTime(row["date_utc"])
		if !ok {
			continue
		}
		payload, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.RaceControlEvent.CreateOne(
			db.RaceControlEvent.SessionID.Set(sessionID),
			db.RaceControlEvent.DateUtc.Set(dateUtc),
			db.RaceControlEvent.Raw.Set(payload),
			db.RaceControlEvent.LapNumber.SetIfPresent(intPtr(row["lap_number"])),
			db.RaceControlEvent.DriverNumber.SetIfPresent(intPtr(row["driver_number"])),
			db.RaceControlEvent.Category.SetIfPresent(stringPtr(row["category"])),
			db.RaceControlEvent.Flag.SetIfPresent(stringPtr(row["flag"])),
			db.RaceControlEvent.Scope.SetIfPresent(stringPtr(row["scope"])),
			db.RaceControlEvent.Message.SetIfPresent(stringPtr(row["message"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// deleteTelemetryScope removes existing scoped rows before replacing them with fresh ingestion data.
func (r *IngestionRepository) deleteTelemetryScope(ctx context.Context, sessionID string, rows []map[string]any) error {
	return deleteByDrivers(ctx, rows, func(driverNumber int) error {
		_, err := r.client.RaceTelemetrySample.FindMany(db.RaceTelemetrySample.SessionID.Equals(sessionID), db.RaceTelemetrySample.DriverNumber.Equals(driverNumber)).Delete().Exec(ctx)
		return err
	})
}

// deleteLocationScope removes existing scoped rows before replacing them with fresh ingestion data.
func (r *IngestionRepository) deleteLocationScope(ctx context.Context, sessionID string, rows []map[string]any) error {
	return deleteByDrivers(ctx, rows, func(driverNumber int) error {
		_, err := r.client.RaceLocationSample.FindMany(db.RaceLocationSample.SessionID.Equals(sessionID), db.RaceLocationSample.DriverNumber.Equals(driverNumber)).Delete().Exec(ctx)
		return err
	})
}

// deletePositionScope removes existing scoped rows before replacing them with fresh ingestion data.
func (r *IngestionRepository) deletePositionScope(ctx context.Context, sessionID string, rows []map[string]any) error {
	return deleteByDrivers(ctx, rows, func(driverNumber int) error {
		_, err := r.client.RacePositionSample.FindMany(db.RacePositionSample.SessionID.Equals(sessionID), db.RacePositionSample.DriverNumber.Equals(driverNumber)).Delete().Exec(ctx)
		return err
	})
}

// deleteIntervalsScope removes existing scoped rows before replacing them with fresh ingestion data.
func (r *IngestionRepository) deleteIntervalsScope(ctx context.Context, sessionID string, rows []map[string]any) error {
	return deleteByDrivers(ctx, rows, func(driverNumber int) error {
		_, err := r.client.RaceIntervalSample.FindMany(db.RaceIntervalSample.SessionID.Equals(sessionID), db.RaceIntervalSample.DriverNumber.Equals(driverNumber)).Delete().Exec(ctx)
		return err
	})
}

// deleteStintsScope removes existing scoped rows before replacing them with fresh ingestion data.
func (r *IngestionRepository) deleteStintsScope(ctx context.Context, sessionID string, rows []map[string]any) error {
	return deleteByDrivers(ctx, rows, func(driverNumber int) error {
		_, err := r.client.RaceStint.FindMany(db.RaceStint.SessionID.Equals(sessionID), db.RaceStint.DriverNumber.Equals(driverNumber)).Delete().Exec(ctx)
		return err
	})
}

// deletePitScope removes existing scoped rows before replacing them with fresh ingestion data.
func (r *IngestionRepository) deletePitScope(ctx context.Context, sessionID string, rows []map[string]any) error {
	return deleteByDrivers(ctx, rows, func(driverNumber int) error {
		_, err := r.client.RacePitStop.FindMany(db.RacePitStop.SessionID.Equals(sessionID), db.RacePitStop.DriverNumber.Equals(driverNumber)).Delete().Exec(ctx)
		return err
	})
}

// deleteRadioScope removes existing scoped rows before replacing them with fresh ingestion data.
func (r *IngestionRepository) deleteRadioScope(ctx context.Context, sessionID string, rows []map[string]any) error {
	return deleteByDrivers(ctx, rows, func(driverNumber int) error {
		_, err := r.client.TeamRadioMessage.FindMany(db.TeamRadioMessage.SessionID.Equals(sessionID), db.TeamRadioMessage.DriverNumber.Equals(driverNumber)).Delete().Exec(ctx)
		return err
	})
}

// deleteByDrivers removes existing scoped rows before replacing them with fresh ingestion data.
func deleteByDrivers(_ context.Context, rows []map[string]any, fn func(driverNumber int) error) error {
	seen := map[int]struct{}{}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		if _, exists := seen[driverNumber]; exists {
			continue
		}
		seen[driverNumber] = struct{}{}
		if err := fn(driverNumber); err != nil {
			return err
		}
	}
	return nil
}

// parseTime parses provider timestamp values into the service DateTime type.
func parseTime(value any) (db.DateTime, bool) {
	text := stringify(value)
	if text == "" {
		return db.DateTime{}, false
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return db.DateTime{}, false
	}
	return db.DateTime(parsed.UTC()), true
}

// parseOptionalTime parses optional provider timestamp values when present.
func parseOptionalTime(value any) *db.DateTime {
	parsed, ok := parseTime(value)
	if !ok {
		return nil
	}
	return &parsed
}

// intValue converts common numeric payload values into an int, returning zero when absent.
func intValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return 0
}

// intPtr converts a dynamic value into an optional int pointer.
func intPtr(value any) *int {
	v := intValue(value)
	if v == 0 {
		return nil
	}
	return &v
}

// floatPtr converts a dynamic value into an optional float pointer.
func floatPtr(value any) *float64 {
	switch v := value.(type) {
	case float64:
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

// boolPtr converts a dynamic value into an optional bool pointer.
func boolPtr(value any) *bool {
	switch v := value.(type) {
	case bool:
		return &v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			return &parsed
		}
	}
	return nil
}

// stringPtr converts a dynamic value into an optional string pointer.
func stringPtr(value any) *string {
	text := stringify(value)
	if text == "" {
		return nil
	}
	return &text
}

// stringify converts dynamic values into their string representation.
func stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.Itoa(int(v))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

// jsonValue converts mapped payload data into the Prisma JSON type.
func jsonValue(value any) (db.JSON, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return db.JSON(encoded), nil
}
