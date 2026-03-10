/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_normalized.go - Normalized session storage, cleanup, and dataset insert helpers.
##
*/

package prisma

import (
	"context"
	"encoding/json"
	"fmt"

	"overdrive/internal/domain"
	"overdrive/resources/db"
)

func (s *RaceArchiveStore) storeNormalizedSessionData(ctx context.Context, sessionID string, archive domain.RaceArchive) error {
	driverNumber := archive.Metadata.DriverNumber
	if driverNumber > 0 {
		if err := s.clearDriverSessionData(ctx, sessionID, driverNumber); err != nil {
			return err
		}
		if err := s.clearSharedSessionData(ctx, sessionID, archive.Datasets); err != nil {
			return err
		}
	} else {
		if err := s.clearSessionData(ctx, sessionID); err != nil {
			return err
		}
	}

	writers := []struct {
		name string
		fn   func(context.Context, string, []map[string]any) error
		rows []map[string]any
	}{
		{name: "laps", fn: s.insertRaceLaps, rows: archive.Datasets["laps"]},
		{name: "car_data", fn: s.insertRaceTelemetry, rows: archive.Datasets["car_data"]},
		{name: "location", fn: s.insertRaceLocations, rows: archive.Datasets["location"]},
		{name: "position", fn: s.insertRacePositions, rows: archive.Datasets["position"]},
		{name: "intervals", fn: s.insertRaceIntervals, rows: archive.Datasets["intervals"]},
		{name: "stints", fn: s.insertRaceStints, rows: archive.Datasets["stints"]},
		{name: "pit", fn: s.insertRacePitStops, rows: archive.Datasets["pit"]},
		{name: "race_control", fn: s.insertRaceControlEvents, rows: archive.Datasets["race_control"]},
		{name: "team_radio", fn: s.insertTeamRadioMessages, rows: archive.Datasets["team_radio"]},
		{name: "weather", fn: s.insertRaceWeatherSamples, rows: archive.Datasets["weather"]},
		{name: "session_result", fn: s.insertSessionResults, rows: archive.Datasets["session_result"]},
		{name: "starting_grid", fn: s.insertStartingGridRows, rows: archive.Datasets["starting_grid"]},
		{name: "overtakes", fn: s.insertOvertakeEvents, rows: archive.Datasets["overtakes"]},
		{name: "championship_drivers", fn: s.insertDriverChampionshipRows, rows: archive.Datasets["championship_drivers"]},
		{name: "championship_teams", fn: s.insertTeamChampionshipRows, rows: archive.Datasets["championship_teams"]},
	}

	for _, writer := range writers {
		if err := writer.fn(ctx, sessionID, writer.rows); err != nil {
			return fmt.Errorf("store dataset %q: %w", writer.name, err)
		}
	}

	return nil
}

// storeNormalizedSessionDataTx builds one transactional batch of normalized session mutations.
func (s *RaceArchiveStore) storeNormalizedSessionDataTx(sessionID string, archive domain.RaceArchive) ([]db.PrismaTransaction, error) {
	queries := make([]db.PrismaTransaction, 0, 32)
	driverNumber := archive.Metadata.DriverNumber

	if driverNumber > 0 {
		queries = append(queries, s.clearDriverSessionDataTx(sessionID, driverNumber)...)
		queries = append(queries, s.clearSharedSessionDataTx(sessionID, archive.Datasets)...)
	} else {
		queries = append(queries, s.clearSessionDataTx(sessionID)...)
	}

	builders := []struct {
		name  string
		query string
		rows  []map[string]any
	}{
		{name: "laps", query: raceLapInsertQuery, rows: archive.Datasets["laps"]},
		{name: "car_data", query: raceTelemetryInsertQuery, rows: archive.Datasets["car_data"]},
		{name: "location", query: raceLocationInsertQuery, rows: archive.Datasets["location"]},
		{name: "position", query: racePositionInsertQuery, rows: archive.Datasets["position"]},
		{name: "intervals", query: raceIntervalInsertQuery, rows: archive.Datasets["intervals"]},
		{name: "stints", query: raceStintInsertQuery, rows: archive.Datasets["stints"]},
		{name: "pit", query: racePitInsertQuery, rows: archive.Datasets["pit"]},
		{name: "race_control", query: raceControlInsertQuery, rows: archive.Datasets["race_control"]},
		{name: "team_radio", query: teamRadioInsertQuery, rows: archive.Datasets["team_radio"]},
		{name: "weather", query: raceWeatherInsertQuery, rows: archive.Datasets["weather"]},
		{name: "session_result", query: sessionResultInsertQuery, rows: archive.Datasets["session_result"]},
		{name: "starting_grid", query: startingGridInsertQuery, rows: archive.Datasets["starting_grid"]},
		{name: "overtakes", query: overtakeInsertQuery, rows: archive.Datasets["overtakes"]},
		{name: "championship_drivers", query: driverChampionshipInsertQuery, rows: archive.Datasets["championship_drivers"]},
		{name: "championship_teams", query: teamChampionshipInsertQuery, rows: archive.Datasets["championship_teams"]},
	}

	for _, builder := range builders {
		query, err := s.executeDatasetInsertTx(sessionID, builder.rows, builder.query)
		if err != nil {
			return nil, fmt.Errorf("store dataset %q: %w", builder.name, err)
		}
		queries = append(queries, query)
	}

	return queries, nil
}

// clearSessionDataTx removes previous normalized rows for a session before reimport.
func (s *RaceArchiveStore) clearSessionDataTx(sessionID string) []db.PrismaTransaction {
	tables := []string{
		"RaceHighlight",
		"RaceLap",
		"RaceTelemetrySample",
		"RaceLocationSample",
		"RacePositionSample",
		"RaceIntervalSample",
		"RaceStint",
		"RacePitStop",
		"RaceControlEvent",
		"TeamRadioMessage",
		"RaceWeatherSample",
		"SessionResultRow",
		"StartingGridRow",
		"OvertakeEvent",
		"DriverChampionshipStanding",
		"TeamChampionshipStanding",
	}

	queries := make([]db.PrismaTransaction, 0, len(tables))
	for _, table := range tables {
		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1`, table)
		queries = append(queries, s.client.Prisma.Raw.ExecuteRaw(query, sessionID).Tx())
	}

	return queries
}

// clearDriverSessionDataTx removes only driver-scoped rows for one driver before reimport.
func (s *RaceArchiveStore) clearDriverSessionDataTx(sessionID string, driverNumber int) []db.PrismaTransaction {
	tables := []string{
		"RaceLap",
		"RaceTelemetrySample",
		"RaceLocationSample",
		"RacePositionSample",
		"RaceIntervalSample",
		"RaceStint",
		"RacePitStop",
		"TeamRadioMessage",
		"SessionResultRow",
		"StartingGridRow",
		"DriverChampionshipStanding",
	}

	queries := make([]db.PrismaTransaction, 0, len(tables))
	for _, table := range tables {
		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1 AND "driverNumber" = $2`, table)
		queries = append(queries, s.client.Prisma.Raw.ExecuteRaw(query, sessionID, driverNumber).Tx())
	}

	return queries
}

// clearSharedSessionDataTx refreshes shared datasets only when the current import includes them.
func (s *RaceArchiveStore) clearSharedSessionDataTx(sessionID string, datasets map[string][]map[string]any) []db.PrismaTransaction {
	tablesByDataset := map[string]string{
		"race_control":       "RaceControlEvent",
		"weather":            "RaceWeatherSample",
		"overtakes":          "OvertakeEvent",
		"championship_teams": "TeamChampionshipStanding",
	}

	queries := make([]db.PrismaTransaction, 0, len(tablesByDataset))
	for dataset, table := range tablesByDataset {
		if len(datasets[dataset]) == 0 {
			continue
		}

		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1`, table)
		queries = append(queries, s.client.Prisma.Raw.ExecuteRaw(query, sessionID).Tx())
	}

	return queries
}

const (
	raceLapInsertQuery = `
		INSERT INTO "RaceLap" (
			"sessionId","driverNumber","lapNumber","dateStartUtc","lapDurationSec",
			"sector1Sec","sector2Sec","sector3Sec","speedI1Kph","speedI2Kph",
			"speedTrapKph","isPitOutLap","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'date_start','')::timestamptz,
			NULLIF(row->>'lap_duration','')::double precision,
			NULLIF(row->>'duration_sector_1','')::double precision,
			NULLIF(row->>'duration_sector_2','')::double precision,
			NULLIF(row->>'duration_sector_3','')::double precision,
			NULLIF(row->>'i1_speed','')::double precision::integer,
			NULLIF(row->>'i2_speed','')::double precision::integer,
			NULLIF(row->>'st_speed','')::double precision::integer,
			CASE WHEN row ? 'is_pit_out_lap' THEN (row->>'is_pit_out_lap')::boolean ELSE NULL END,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'lap_number','') IS NOT NULL
	`
	raceTelemetryInsertQuery = `
		INSERT INTO "RaceTelemetrySample" (
			"sessionId","driverNumber","dateUtc","speedKph","rpm","gear",
			"throttlePct","brakePct","drsState","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'speed','')::double precision::integer,
			NULLIF(row->>'rpm','')::double precision::integer,
			NULLIF(row->>'n_gear','')::double precision::integer,
			NULLIF(row->>'throttle','')::double precision,
			NULLIF(row->>'brake','')::double precision,
			NULLIF(row->>'drs','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`
	raceLocationInsertQuery = `
		INSERT INTO "RaceLocationSample" (
			"sessionId","driverNumber","dateUtc","x","y","z","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'x','')::double precision,
			NULLIF(row->>'y','')::double precision,
			NULLIF(row->>'z','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`
	racePositionInsertQuery = `
		INSERT INTO "RacePositionSample" (
			"sessionId","driverNumber","dateUtc","lapNumber","position","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'position','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`
	raceIntervalInsertQuery = `
		INSERT INTO "RaceIntervalSample" (
			"sessionId","driverNumber","dateUtc","gapToLeader","intervalToFrontDriver","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'gap_to_leader',''),
			COALESCE(NULLIF(row->>'interval_to_front',''), NULLIF(row->>'interval','')),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`
	raceStintInsertQuery = `
		INSERT INTO "RaceStint" (
			"sessionId","driverNumber","stintNumber","lapStart","lapEnd",
			"compound","tyreAgeLapsStart","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'stint_number','')::double precision::integer,
			NULLIF(row->>'lap_start','')::double precision::integer,
			NULLIF(row->>'lap_end','')::double precision::integer,
			NULLIF(row->>'compound',''),
			COALESCE(
				NULLIF(row->>'tyre_age_at_start','')::double precision::integer,
				NULLIF(row->>'tyre_age_at_start_lap','')::double precision::integer
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`
	racePitInsertQuery = `
		INSERT INTO "RacePitStop" (
			"sessionId","driverNumber","lapNumber","dateUtc","pitDurationSec","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			COALESCE(
				NULLIF(row->>'pit_duration','')::double precision,
				NULLIF(row->>'pit_duration_sec','')::double precision
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`
	raceControlInsertQuery = `
		INSERT INTO "RaceControlEvent" (
			"sessionId","dateUtc","lapNumber","driverNumber","category","flag","scope","message","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'category',''),
			NULLIF(row->>'flag',''),
			NULLIF(row->>'scope',''),
			NULLIF(row->>'message',''),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'date','') IS NOT NULL
	`
	teamRadioInsertQuery = `
		INSERT INTO "TeamRadioMessage" (
			"sessionId","driverNumber","dateUtc","recordingUrl","transcript","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'recording_url',''),
			NULLIF(row->>'transcript',''),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`
	raceWeatherInsertQuery = `
		INSERT INTO "RaceWeatherSample" (
			"sessionId","dateUtc","airTempC","trackTempC","humidityPct","pressureHpa",
			"windSpeedKph","windDirectionDeg","rainfallMm","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'air_temperature','')::double precision,
			NULLIF(row->>'track_temperature','')::double precision,
			NULLIF(row->>'humidity','')::double precision,
			NULLIF(row->>'pressure','')::double precision,
			NULLIF(row->>'wind_speed','')::double precision,
			NULLIF(row->>'wind_direction','')::double precision::integer,
			NULLIF(row->>'rainfall','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'date','') IS NOT NULL
	`
	sessionResultInsertQuery = `
		INSERT INTO "SessionResultRow" (
			"sessionId","driverNumber","position","points","status","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'position','')::double precision::integer,
			NULLIF(row->>'points','')::double precision,
			COALESCE(
				NULLIF(row->>'status',''),
				NULLIF(row->>'classification_status',''),
				CASE WHEN lower(COALESCE(row->>'dnf','false')) = 'true' THEN 'DNF' ELSE NULL END
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`
	startingGridInsertQuery = `
		INSERT INTO "StartingGridRow" (
			"sessionId","driverNumber","gridPosition","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			COALESCE(
				NULLIF(row->>'grid_position','')::double precision::integer,
				NULLIF(row->>'position','')::double precision::integer
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`
	overtakeInsertQuery = `
		INSERT INTO "OvertakeEvent" (
			"sessionId","dateUtc","lapNumber","overtakingDriverNumber","overtakenDriverNumber","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			COALESCE(
				NULLIF(row->>'overtaking_driver_number','')::double precision::integer,
				NULLIF(row->>'driver_number','')::double precision::integer
			),
			NULLIF(row->>'overtaken_driver_number','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
	`
	driverChampionshipInsertQuery = `
		INSERT INTO "DriverChampionshipStanding" (
			"sessionId","driverNumber","positionCurrent","positionStart","pointsCurrent","pointsStart","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'position_current','')::double precision::integer,
			NULLIF(row->>'position_start','')::double precision::integer,
			NULLIF(row->>'points_current','')::double precision,
			NULLIF(row->>'points_start','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`
	teamChampionshipInsertQuery = `
		INSERT INTO "TeamChampionshipStanding" (
			"sessionId","teamName","teamCode","positionCurrent","positionStart","pointsCurrent","pointsStart","raw"
		)
		SELECT
			$1,
			COALESCE(NULLIF(row->>'team_name',''), NULLIF(row->>'constructor_name','')),
			COALESCE(NULLIF(row->>'team_code',''), NULLIF(row->>'name_acronym','')),
			NULLIF(row->>'position_current','')::double precision::integer,
			NULLIF(row->>'position_start','')::double precision::integer,
			NULLIF(row->>'points_current','')::double precision,
			NULLIF(row->>'points_start','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
	`
)

// clearSessionData removes previous normalized rows for a session before reimport.
func (s *RaceArchiveStore) clearSessionData(ctx context.Context, sessionID string) error {
	tables := []string{
		"RaceHighlight",
		"RaceLap",
		"RaceTelemetrySample",
		"RaceLocationSample",
		"RacePositionSample",
		"RaceIntervalSample",
		"RaceStint",
		"RacePitStop",
		"RaceControlEvent",
		"TeamRadioMessage",
		"RaceWeatherSample",
		"SessionResultRow",
		"StartingGridRow",
		"OvertakeEvent",
		"DriverChampionshipStanding",
		"TeamChampionshipStanding",
	}

	for _, table := range tables {
		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1`, table)
		if _, err := s.client.Prisma.Raw.ExecuteRaw(query, sessionID).Exec(ctx); err != nil {
			return fmt.Errorf("clear table %s: %w", table, err)
		}
	}

	return nil
}

// clearDriverSessionData removes only driver-scoped rows for one driver before reimport.
func (s *RaceArchiveStore) clearDriverSessionData(ctx context.Context, sessionID string, driverNumber int) error {
	tables := []string{
		"RaceLap",
		"RaceTelemetrySample",
		"RaceLocationSample",
		"RacePositionSample",
		"RaceIntervalSample",
		"RaceStint",
		"RacePitStop",
		"TeamRadioMessage",
		"SessionResultRow",
		"StartingGridRow",
		"DriverChampionshipStanding",
	}

	for _, table := range tables {
		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1 AND "driverNumber" = $2`, table)
		if _, err := s.client.Prisma.Raw.ExecuteRaw(query, sessionID, driverNumber).Exec(ctx); err != nil {
			return fmt.Errorf("clear driver table %s: %w", table, err)
		}
	}

	return nil
}

// clearSharedSessionData refreshes shared datasets only when the current import includes them.
func (s *RaceArchiveStore) clearSharedSessionData(ctx context.Context, sessionID string, datasets map[string][]map[string]any) error {
	tablesByDataset := map[string]string{
		"race_control":       "RaceControlEvent",
		"weather":            "RaceWeatherSample",
		"overtakes":          "OvertakeEvent",
		"championship_teams": "TeamChampionshipStanding",
	}

	for dataset, table := range tablesByDataset {
		if len(datasets[dataset]) == 0 {
			continue
		}

		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1`, table)
		if _, err := s.client.Prisma.Raw.ExecuteRaw(query, sessionID).Exec(ctx); err != nil {
			return fmt.Errorf("clear shared table %s: %w", table, err)
		}
	}

	return nil
}

// insertRaceLaps stores lap rows in the normalized race lap table.
func (s *RaceArchiveStore) insertRaceLaps(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceLap" (
			"sessionId","driverNumber","lapNumber","dateStartUtc","lapDurationSec",
			"sector1Sec","sector2Sec","sector3Sec","speedI1Kph","speedI2Kph",
			"speedTrapKph","isPitOutLap","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'date_start','')::timestamptz,
			NULLIF(row->>'lap_duration','')::double precision,
			NULLIF(row->>'duration_sector_1','')::double precision,
			NULLIF(row->>'duration_sector_2','')::double precision,
			NULLIF(row->>'duration_sector_3','')::double precision,
			NULLIF(row->>'i1_speed','')::double precision::integer,
			NULLIF(row->>'i2_speed','')::double precision::integer,
			NULLIF(row->>'st_speed','')::double precision::integer,
			CASE WHEN row ? 'is_pit_out_lap' THEN (row->>'is_pit_out_lap')::boolean ELSE NULL END,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'lap_number','') IS NOT NULL
	`)
}

// insertRaceTelemetry stores car_data rows in the telemetry table.
func (s *RaceArchiveStore) insertRaceTelemetry(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceTelemetrySample" (
			"sessionId","driverNumber","dateUtc","speedKph","rpm","gear",
			"throttlePct","brakePct","drsState","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'speed','')::double precision::integer,
			NULLIF(row->>'rpm','')::double precision::integer,
			NULLIF(row->>'n_gear','')::double precision::integer,
			NULLIF(row->>'throttle','')::double precision,
			NULLIF(row->>'brake','')::double precision,
			NULLIF(row->>'drs','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertRaceLocations stores location rows in the normalized location table.
func (s *RaceArchiveStore) insertRaceLocations(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceLocationSample" (
			"sessionId","driverNumber","dateUtc","x","y","z","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'x','')::double precision,
			NULLIF(row->>'y','')::double precision,
			NULLIF(row->>'z','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertRacePositions stores in-race positions in the normalized position table.
func (s *RaceArchiveStore) insertRacePositions(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RacePositionSample" (
			"sessionId","driverNumber","dateUtc","lapNumber","position","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'position','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertRaceIntervals stores interval rows in the normalized interval table.
func (s *RaceArchiveStore) insertRaceIntervals(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceIntervalSample" (
			"sessionId","driverNumber","dateUtc","gapToLeader","intervalToFrontDriver","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'gap_to_leader',''),
			COALESCE(NULLIF(row->>'interval_to_front',''), NULLIF(row->>'interval','')),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertRaceStints stores stint rows in the normalized stint table.
func (s *RaceArchiveStore) insertRaceStints(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceStint" (
			"sessionId","driverNumber","stintNumber","lapStart","lapEnd",
			"compound","tyreAgeLapsStart","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'stint_number','')::double precision::integer,
			NULLIF(row->>'lap_start','')::double precision::integer,
			NULLIF(row->>'lap_end','')::double precision::integer,
			NULLIF(row->>'compound',''),
			COALESCE(
				NULLIF(row->>'tyre_age_at_start','')::double precision::integer,
				NULLIF(row->>'tyre_age_at_start_lap','')::double precision::integer
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertRacePitStops stores pit rows in the normalized pit stop table.
func (s *RaceArchiveStore) insertRacePitStops(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RacePitStop" (
			"sessionId","driverNumber","lapNumber","dateUtc","pitDurationSec","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			COALESCE(
				NULLIF(row->>'pit_duration','')::double precision,
				NULLIF(row->>'pit_duration_sec','')::double precision
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertRaceControlEvents stores race control rows in the normalized race control table.
func (s *RaceArchiveStore) insertRaceControlEvents(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceControlEvent" (
			"sessionId","dateUtc","lapNumber","driverNumber","category","flag","scope","message","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'category',''),
			NULLIF(row->>'flag',''),
			NULLIF(row->>'scope',''),
			NULLIF(row->>'message',''),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertTeamRadioMessages stores radio rows in the normalized team radio table.
func (s *RaceArchiveStore) insertTeamRadioMessages(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "TeamRadioMessage" (
			"sessionId","driverNumber","dateUtc","recordingUrl","transcript","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'recording_url',''),
			NULLIF(row->>'transcript',''),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertRaceWeatherSamples stores weather rows in the normalized weather table.
func (s *RaceArchiveStore) insertRaceWeatherSamples(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceWeatherSample" (
			"sessionId","dateUtc","airTempC","trackTempC","humidityPct","pressureHpa",
			"windSpeedKph","windDirectionDeg","rainfallMm","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'air_temperature','')::double precision,
			NULLIF(row->>'track_temperature','')::double precision,
			NULLIF(row->>'humidity','')::double precision,
			NULLIF(row->>'pressure','')::double precision,
			NULLIF(row->>'wind_speed','')::double precision,
			NULLIF(row->>'wind_direction','')::double precision::integer,
			NULLIF(row->>'rainfall','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertSessionResults stores session result rows in the normalized results table.
func (s *RaceArchiveStore) insertSessionResults(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "SessionResultRow" (
			"sessionId","driverNumber","position","points","status","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'position','')::double precision::integer,
			NULLIF(row->>'points','')::double precision,
			COALESCE(
				NULLIF(row->>'status',''),
				NULLIF(row->>'classification_status',''),
				CASE WHEN lower(COALESCE(row->>'dnf','false')) = 'true' THEN 'DNF' ELSE NULL END
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertStartingGridRows stores starting grid rows in the normalized starting grid table.
func (s *RaceArchiveStore) insertStartingGridRows(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "StartingGridRow" (
			"sessionId","driverNumber","gridPosition","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			COALESCE(
				NULLIF(row->>'grid_position','')::double precision::integer,
				NULLIF(row->>'position','')::double precision::integer
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertOvertakeEvents stores overtakes in the normalized overtake table.
func (s *RaceArchiveStore) insertOvertakeEvents(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "OvertakeEvent" (
			"sessionId","dateUtc","lapNumber","overtakingDriverNumber","overtakenDriverNumber","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			COALESCE(
				NULLIF(row->>'overtaking_driver_number','')::double precision::integer,
				NULLIF(row->>'driver_number','')::double precision::integer
			),
			NULLIF(row->>'overtaken_driver_number','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
	`)
}

// insertDriverChampionshipRows stores driver championship standings in the normalized table.
func (s *RaceArchiveStore) insertDriverChampionshipRows(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "DriverChampionshipStanding" (
			"sessionId","driverNumber","positionCurrent","positionStart","pointsCurrent","pointsStart","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'position_current','')::double precision::integer,
			NULLIF(row->>'position_start','')::double precision::integer,
			NULLIF(row->>'points_current','')::double precision,
			NULLIF(row->>'points_start','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertTeamChampionshipRows stores constructor championship standings in the normalized table.
func (s *RaceArchiveStore) insertTeamChampionshipRows(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "TeamChampionshipStanding" (
			"sessionId","teamName","teamCode","positionCurrent","positionStart","pointsCurrent","pointsStart","raw"
		)
		SELECT
			$1,
			COALESCE(NULLIF(row->>'team_name',''), NULLIF(row->>'constructor_name','')),
			COALESCE(NULLIF(row->>'team_code',''), NULLIF(row->>'name_acronym','')),
			NULLIF(row->>'position_current','')::double precision::integer,
			NULLIF(row->>'position_start','')::double precision::integer,
			NULLIF(row->>'points_current','')::double precision,
			NULLIF(row->>'points_start','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
	`)
}

// executeDatasetInsert runs a bulk insert SQL statement from a dataset JSON payload.
func (s *RaceArchiveStore) executeDatasetInsert(ctx context.Context, sessionID string, rows []map[string]any, query string) error {
	payload, err := json.Marshal(nonNilRows(rows))
	if err != nil {
		return fmt.Errorf("marshal dataset rows: %w", err)
	}

	if _, err := s.client.Prisma.Raw.ExecuteRaw(query, sessionID, string(payload)).Exec(ctx); err != nil {
		return err
	}

	return nil
}

// executeDatasetInsertTx builds a bulk insert transaction from a dataset JSON payload.
func (s *RaceArchiveStore) executeDatasetInsertTx(sessionID string, rows []map[string]any, query string) (db.PrismaTransaction, error) {
	payload, err := json.Marshal(nonNilRows(rows))
	if err != nil {
		return nil, fmt.Errorf("marshal dataset rows: %w", err)
	}

	return s.client.Prisma.Raw.ExecuteRaw(query, sessionID, string(payload)).Tx(), nil
}
