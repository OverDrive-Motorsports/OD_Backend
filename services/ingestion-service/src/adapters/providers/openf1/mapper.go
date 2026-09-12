/**
##
## OverDrive 2026
## All Technical rights reserved
##
## mapper.go - Package openf1 source file for services/ingestion-service/src/adapters/providers/openf1.
##
*/

package openf1

import (
	"fmt"
	"strings"

	"overdrive/services/ingestion-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

type Mapper struct{}

// NewMapper builds and returns a mapper with its required dependencies.
func NewMapper() *Mapper {
	return &Mapper{}
}

// SupportedResources returns the provider resources supported by this mapper.
func (m *Mapper) SupportedResources() []string {
	return []string{
		"meetings",
		"sessions",
		"drivers",
		"championship_drivers",
		"championship_teams",
		"session_result",
		"starting_grid",
		"laps",
		"car_data",
		"location",
		"position",
		"intervals",
		"stints",
		"pit",
		"weather",
		"team_radio",
		"overtakes",
		"race_control",
	}
}

// Map converts provider rows into the internal ingestion dataset format.
func (m *Mapper) Map(resource string, rows []map[string]any) (contracts.Dataset, error) {
	switch resource {
	case "meetings":
		return m.mapMeetings(rows), nil
	case "sessions":
		return m.mapSessions(rows), nil
	case "drivers":
		return m.mapDrivers(rows), nil
	case "championship_drivers":
		return m.mapChampionshipDrivers(rows), nil
	case "championship_teams":
		return m.mapChampionshipTeams(rows), nil
	case "session_result":
		return m.mapSessionResult(rows), nil
	case "starting_grid":
		return m.mapStartingGrid(rows), nil
	case "laps":
		return m.mapLaps(rows), nil
	case "car_data":
		return m.mapTelemetry(rows), nil
	case "location":
		return m.mapLocation(rows), nil
	case "position":
		return m.mapPosition(rows), nil
	case "intervals":
		return m.mapIntervals(rows), nil
	case "stints":
		return m.mapStints(rows), nil
	case "pit":
		return m.mapPit(rows), nil
	case "weather":
		return m.mapWeather(rows), nil
	case "team_radio":
		return m.mapTeamRadio(rows), nil
	case "overtakes":
		return m.mapOvertakes(rows), nil
	case "race_control":
		return m.mapRaceControl(rows), nil
	default:
		return contracts.Dataset{}, fmt.Errorf("%w %q", domain.ErrUnsupportedResource, resource)
	}
}

// mapMeetings maps OpenF1 meetings rows into an internal ingestion dataset.
func (m *Mapper) mapMeetings(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"meeting_key":           row["meeting_key"],
			"season_year":           row["year"],
			"meeting_name":          row["meeting_name"],
			"meeting_official_name": row["meeting_official_name"],
			"location":              row["location"],
			"country_name":          row["country_name"],
			"country_code":          row["country_code"],
			"circuit_name":          row["circuit_short_name"],
			"start_time_utc":        row["date_start"],
			"end_time_utc":          row["date_end"],
			"raw":                   row,
		})
	}
	return newDataset("event_catalog", "catalog", "championship-service", "meetings", mapped)
}

// mapSessions maps OpenF1 sessions rows into an internal ingestion dataset.
func (m *Mapper) mapSessions(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":    row["session_key"],
			"meeting_key":    row["meeting_key"],
			"session_name":   row["session_name"],
			"session_type":   strings.ToLower(asString(row["session_type"])),
			"start_time_utc": row["date_start"],
			"end_time_utc":   row["date_end"],
			"location":       row["location"],
			"country_name":   row["country_name"],
			"country_code":   row["country_code"],
			"circuit_name":   row["circuit_short_name"],
			"raw":            row,
		})
	}
	return newDataset("session_catalog", "catalog", "championship-service", "sessions", mapped)
}

// mapDrivers maps OpenF1 drivers rows into an internal ingestion dataset.
func (m *Mapper) mapDrivers(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":    row["session_key"],
			"meeting_key":    row["meeting_key"],
			"driver_number":  row["driver_number"],
			"display_name":   row["full_name"],
			"first_name":     row["first_name"],
			"last_name":      row["last_name"],
			"driver_code":    row["name_acronym"],
			"country_code":   row["country_code"],
			"team_name":      row["team_name"],
			"team_color_hex": row["team_colour"],
			"headshot_url":   row["headshot_url"],
			"raw":            row,
		})
	}
	return newDataset("driver_catalog", "catalog", "championship-service", "drivers", mapped)
}

// mapChampionshipDrivers maps OpenF1 championship drivers rows into an internal ingestion dataset.
func (m *Mapper) mapChampionshipDrivers(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":      row["session_key"],
			"meeting_key":      row["meeting_key"],
			"driver_number":    row["driver_number"],
			"points_current":   row["points_current"],
			"points_start":     row["points_start"],
			"position_current": row["position_current"],
			"position_start":   row["position_start"],
			"raw":              row,
		})
	}
	return newDataset("driver_championship_standings", "catalog", "championship-service", "championship_drivers", mapped)
}

// mapChampionshipTeams maps OpenF1 championship teams rows into an internal ingestion dataset.
func (m *Mapper) mapChampionshipTeams(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":      row["session_key"],
			"meeting_key":      row["meeting_key"],
			"team_name":        row["team_name"],
			"points_current":   row["points_current"],
			"points_start":     row["points_start"],
			"position_current": row["position_current"],
			"position_start":   row["position_start"],
			"raw":              row,
		})
	}
	return newDataset("team_championship_standings", "catalog", "championship-service", "championship_teams", mapped)
}

// mapSessionResult maps OpenF1 session result rows into an internal ingestion dataset.
func (m *Mapper) mapSessionResult(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":    row["session_key"],
			"meeting_key":    row["meeting_key"],
			"driver_number":  row["driver_number"],
			"position":       row["position"],
			"points":         row["points"],
			"status":         row["status"],
			"dnf":            row["dnf"],
			"dns":            row["dns"],
			"dsq":            row["dsq"],
			"duration":       row["duration"],
			"gap_to_leader":  row["gap_to_leader"],
			"number_of_laps": row["number_of_laps"],
			"raw":            row,
		})
	}
	return newDataset("session_result", "catalog", "championship-service", "session_result", mapped)
}

// mapStartingGrid maps OpenF1 starting grid rows into an internal ingestion dataset.
func (m *Mapper) mapStartingGrid(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":   row["session_key"],
			"meeting_key":   row["meeting_key"],
			"driver_number": row["driver_number"],
			"position":      row["position"],
			"lap_duration":  row["lap_duration"],
			"raw":           row,
		})
	}
	return newDataset("starting_grid", "catalog", "championship-service", "starting_grid", mapped)
}

// mapLaps maps OpenF1 laps rows into an internal ingestion dataset.
func (m *Mapper) mapLaps(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":      row["session_key"],
			"meeting_key":      row["meeting_key"],
			"driver_number":    row["driver_number"],
			"lap_number":       row["lap_number"],
			"started_at_utc":   row["date_start"],
			"lap_duration_sec": row["lap_duration"],
			"sector_1_sec":     row["duration_sector_1"],
			"sector_2_sec":     row["duration_sector_2"],
			"sector_3_sec":     row["duration_sector_3"],
			"speed_i1_kph":     row["i1_speed"],
			"speed_i2_kph":     row["i2_speed"],
			"speed_trap_kph":   row["st_speed"],
			"is_pit_out_lap":   row["is_pit_out_lap"],
			"raw":              row,
		})
	}
	return newDataset("lap_timing", "race_data", "race-data-service", "laps", mapped)
}

// mapTelemetry maps OpenF1 telemetry rows into an internal ingestion dataset.
func (m *Mapper) mapTelemetry(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":     row["session_key"],
			"meeting_key":     row["meeting_key"],
			"driver_number":   row["driver_number"],
			"sample_time_utc": row["date"],
			"speed_kph":       row["speed"],
			"rpm":             row["rpm"],
			"gear":            row["n_gear"],
			"throttle_pct":    row["throttle"],
			"brake_pct":       row["brake"],
			"drs_state":       row["drs"],
			"raw":             row,
		})
	}
	return newDataset("telemetry", "race_data", "race-data-service", "car_data", mapped)
}

// mapLocation maps OpenF1 location rows into an internal ingestion dataset.
func (m *Mapper) mapLocation(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":     row["session_key"],
			"meeting_key":     row["meeting_key"],
			"driver_number":   row["driver_number"],
			"sample_time_utc": row["date"],
			"x":               row["x"],
			"y":               row["y"],
			"z":               row["z"],
			"raw":             row,
		})
	}
	return newDataset("location", "race_data", "race-data-service", "location", mapped)
}

// mapPosition maps OpenF1 position rows into an internal ingestion dataset.
func (m *Mapper) mapPosition(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":     row["session_key"],
			"meeting_key":     row["meeting_key"],
			"driver_number":   row["driver_number"],
			"sample_time_utc": row["date"],
			"position":        row["position"],
			"raw":             row,
		})
	}
	return newDataset("position", "race_data", "race-data-service", "position", mapped)
}

// mapIntervals maps OpenF1 intervals rows into an internal ingestion dataset.
func (m *Mapper) mapIntervals(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":              row["session_key"],
			"meeting_key":              row["meeting_key"],
			"driver_number":            row["driver_number"],
			"sample_time_utc":          row["date"],
			"gap_to_leader":            row["gap_to_leader"],
			"interval_to_front_driver": row["interval_to_front"],
			"raw":                      row,
		})
	}
	return newDataset("intervals", "race_data", "race-data-service", "intervals", mapped)
}

// mapStints maps OpenF1 stints rows into an internal ingestion dataset.
func (m *Mapper) mapStints(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":         row["session_key"],
			"meeting_key":         row["meeting_key"],
			"driver_number":       row["driver_number"],
			"stint_number":        row["stint_number"],
			"lap_start":           row["lap_start"],
			"lap_end":             row["lap_end"],
			"compound":            row["compound"],
			"tyre_age_laps_start": row["tyre_age_at_start"],
			"raw":                 row,
		})
	}
	return newDataset("stints", "race_data", "race-data-service", "stints", mapped)
}

// mapPit maps OpenF1 pit rows into an internal ingestion dataset.
func (m *Mapper) mapPit(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":      row["session_key"],
			"meeting_key":      row["meeting_key"],
			"driver_number":    row["driver_number"],
			"lap_number":       row["lap_number"],
			"date_utc":         row["date"],
			"pit_duration_sec": row["pit_duration"],
			"raw":              row,
		})
	}
	return newDataset("pit_stops", "race_data", "race-data-service", "pit", mapped)
}

// mapWeather maps OpenF1 weather rows into an internal ingestion dataset.
func (m *Mapper) mapWeather(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":        row["session_key"],
			"meeting_key":        row["meeting_key"],
			"sample_time_utc":    row["date"],
			"air_temp_c":         row["air_temperature"],
			"track_temp_c":       row["track_temperature"],
			"humidity_pct":       row["humidity"],
			"pressure_hpa":       row["pressure"],
			"wind_speed_kph":     row["wind_speed"],
			"wind_direction_deg": row["wind_direction"],
			"rainfall_mm":        row["rainfall"],
			"raw":                row,
		})
	}
	return newDataset("weather", "race_data", "race-data-service", "weather", mapped)
}

// mapTeamRadio maps OpenF1 team radio rows into an internal ingestion dataset.
func (m *Mapper) mapTeamRadio(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":   row["session_key"],
			"meeting_key":   row["meeting_key"],
			"driver_number": row["driver_number"],
			"date_utc":      row["date"],
			"recording_url": row["recording_url"],
			"raw":           row,
		})
	}
	return newDataset("team_radio", "race_data", "race-data-service", "team_radio", mapped)
}

// mapOvertakes maps OpenF1 overtakes rows into an internal ingestion dataset.
func (m *Mapper) mapOvertakes(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":              row["session_key"],
			"meeting_key":              row["meeting_key"],
			"date_utc":                 row["date"],
			"lap_number":               row["lap_number"],
			"overtaking_driver_number": row["overtaking_driver_number"],
			"overtaken_driver_number":  row["overtaken_driver_number"],
			"position":                 row["position"],
			"raw":                      row,
		})
	}
	return newDataset("overtakes", "race_data", "race-data-service", "overtakes", mapped)
}

// mapRaceControl maps OpenF1 race control rows into an internal ingestion dataset.
func (m *Mapper) mapRaceControl(rows []map[string]any) contracts.Dataset {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, map[string]any{
			"session_key":   row["session_key"],
			"meeting_key":   row["meeting_key"],
			"date_utc":      row["date"],
			"lap_number":    row["lap_number"],
			"driver_number": row["driver_number"],
			"category":      row["category"],
			"flag":          row["flag"],
			"scope":         row["scope"],
			"message":       row["message"],
			"raw":           row,
		})
	}
	return newDataset("race_control", "race_data", "race-data-service", "race_control", mapped)
}

// newDataset implements the new dataset workflow for this package.
func newDataset(name string, category string, targetService string, providerResource string, rows []map[string]any) contracts.Dataset {
	return contracts.Dataset{
		Name:             name,
		Category:         category,
		TargetService:    targetService,
		ProviderResource: providerResource,
		SchemaVersion:    "v1",
		RowCount:         len(rows),
		Rows:             rows,
	}
}

// asString converts a dynamic value to a string while preserving empty values.
func asString(value any) string {
	if value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Sprintf("%v", value)
	}
	return text
}
