/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_merge.repository.go - Merges live ingestion batches without duplicating samples.
##
*/

package prismaadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	db "overdrive/services/race-data-service/resources/db"
	contracts "overdrive/shared/contracts/ingestion"
)

// mergeChunkRows bounds the rows of one merge statement.
const mergeChunkRows = 1000

// mergeLockQuery serialises the merges of one table and session, so two overlapping batches
// (two live jobs, a retry after a timeout) cannot both see a row as missing.
const mergeLockQuery = `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`

// mergeTimeLayout is the UTC form sent for a "timestamp" recordset column. The columns are
// timestamp(3): values are truncated to the millisecond first.
const mergeTimeLayout = "2006-01-02T15:04:05.000"

// mergeColumn is one column of a merged table. The recordset column has the same name.
type mergeColumn struct {
	name    string
	sqlType string
	// value reads the column from a mapped row; nil stores NULL.
	value func(row map[string]any) any
	// required drops the rows without a value.
	required bool
}

// mergeTable describes how the rows of a dataset are merged. Names are Prisma names, never input.
type mergeTable struct {
	table   string
	columns []mergeColumn
	// identity names the columns that identify a row within its session.
	identity []string
	// update replaces a stored row with the same identity (a row that changes while the session
	// runs); otherwise the stored row is kept.
	update bool
	// report returns the inserted rows to the caller.
	report bool
}

// mergeTables are the datasets a live batch may hold, besides lap_timing (an upsert already).
var mergeTables = map[string]mergeTable{
	"telemetry": {table: "RaceTelemetrySample", identity: []string{"driverNumber", "dateUtc"}, columns: []mergeColumn{
		mergeDriver(true), mergeTime("sample_time_utc"),
		mergeInt("speedKph", "speed_kph"), mergeInt("rpm", "rpm"), mergeInt("gear", "gear"),
		mergeFloat("throttlePct", "throttle_pct"), mergeFloat("brakePct", "brake_pct"), mergeInt("drsState", "drs_state"),
	}},
	"location": {table: "RaceLocationSample", identity: []string{"driverNumber", "dateUtc"}, columns: []mergeColumn{
		mergeDriver(true), mergeTime("sample_time_utc"),
		mergeFloat("x", "x"), mergeFloat("y", "y"), mergeFloat("z", "z"),
	}},
	"position": {table: "RacePositionSample", identity: []string{"driverNumber", "dateUtc"}, columns: []mergeColumn{
		mergeDriver(true), mergeTime("sample_time_utc"),
		mergeInt("lapNumber", "lap_number"), mergeInt("position", "position"),
	}},
	"intervals": {table: "RaceIntervalSample", identity: []string{"driverNumber", "dateUtc"}, columns: []mergeColumn{
		mergeDriver(true), mergeTime("sample_time_utc"),
		mergeText("gapToLeader", "gap_to_leader", 40), mergeText("intervalToFrontDriver", "interval_to_front_driver", 40),
	}},
	"stints": {table: "RaceStint", identity: []string{"driverNumber", "stintNumber"}, update: true, columns: []mergeColumn{
		mergeDriver(true), mergeInt("stintNumber", "stint_number"), mergeInt("lapStart", "lap_start"), mergeInt("lapEnd", "lap_end"),
		mergeText("compound", "compound", 40), mergeInt("tyreAgeLapsStart", "tyre_age_laps_start"),
	}},
	"pit_stops": {table: "RacePitStop", identity: []string{"driverNumber", "dateUtc"}, update: true, columns: []mergeColumn{
		mergeDriver(true), mergeTime("date_utc"),
		mergeInt("lapNumber", "lap_number"), mergeFloat("pitDurationSec", "pit_duration_sec"),
	}},
	"weather": {table: "RaceWeatherSample", identity: []string{"dateUtc"}, columns: []mergeColumn{
		mergeTime("sample_time_utc"),
		mergeFloat("airTempC", "air_temp_c"), mergeFloat("trackTempC", "track_temp_c"), mergeFloat("humidityPct", "humidity_pct"),
		mergeFloat("pressureHpa", "pressure_hpa"), mergeFloat("windSpeedKph", "wind_speed_kph"),
		mergeInt("windDirectionDeg", "wind_direction_deg"), mergeFloat("rainfallMm", "rainfall_mm"),
	}},
	"team_radio": {table: "TeamRadioMessage", identity: []string{"driverNumber", "dateUtc"}, columns: []mergeColumn{
		mergeDriver(true), mergeTime("date_utc"), mergeText("recordingUrl", "recording_url", 500),
	}},
	"race_control": {table: "RaceControlEvent", identity: []string{"dateUtc", "driverNumber", "category", "flag", "scope", "message"}, report: true, columns: []mergeColumn{
		mergeTime("date_utc"), mergeInt("lapNumber", "lap_number"), mergeDriver(false),
		mergeText("category", "category", 60), mergeText("flag", "flag", 40), mergeText("scope", "scope", 80), mergeText("message", "message", 0),
	}},
}

// MergeBatch adds the rows of a live batch that are not stored yet (see the port). Each chunk of
// a dataset is one transaction; a failure leaves the previous chunks stored, and sending the
// batch again is harmless.
func (r *IngestionRepository) MergeBatch(ctx context.Context, batch contracts.Batch) ([]map[string]any, error) {
	sessionID := contracts.SessionID(batch.Provider, batch.Context.SessionKey)
	var raceControl []map[string]any
	for _, dataset := range batch.Datasets {
		if dataset.Name == "lap_timing" {
			if err := r.storeLaps(ctx, sessionID, dataset.Rows); err != nil {
				return raceControl, fmt.Errorf("merge lap_timing: %w", err)
			}
			continue
		}
		table, ok := mergeTables[dataset.Name]
		if !ok {
			return raceControl, fmt.Errorf("dataset %q cannot be merged", dataset.Name)
		}
		added, err := r.mergeRows(ctx, sessionID, table, dataset.Rows)
		if table.report {
			raceControl = append(raceControl, added...)
		}
		if err != nil {
			return raceControl, fmt.Errorf("merge %s: %w", dataset.Name, err)
		}
	}
	return raceControl, nil
}

// mergeRows merges the rows of one dataset and returns the rows it inserted for a reported table.
func (r *IngestionRepository) mergeRows(ctx context.Context, sessionID string, table mergeTable, rows []map[string]any) ([]map[string]any, error) {
	records := mergeRecords(table, rows)
	var added []map[string]any
	for start := 0; start < len(records); start += mergeChunkRows {
		chunk := records[start:min(start+mergeChunkRows, len(records))]
		payload, err := json.Marshal(chunk)
		if err != nil {
			return added, err
		}
		statements := []db.PrismaTransaction{r.client.Prisma.ExecuteRaw(mergeLockQuery, "race-merge:"+table.table+":"+sessionID).Tx()}
		if table.update {
			statements = append(statements, r.client.Prisma.ExecuteRaw(mergeDeleteQuery(table), sessionID, string(payload)).Tx())
		}
		if !table.report {
			statements = append(statements, r.client.Prisma.ExecuteRaw(mergeInsertQuery(table), sessionID, string(payload)).Tx())
			if err := r.client.Prisma.Transaction(statements...).Exec(ctx); err != nil {
				return added, err
			}
			continue
		}
		insert := r.client.Prisma.QueryRaw(mergeInsertQuery(table), sessionID, string(payload)).Tx()
		if err := r.client.Prisma.Transaction(append(statements, insert)...).Exec(ctx); err != nil {
			return added, err
		}
		var inserted []struct {
			Raw string `json:"raw"`
		}
		if err := insert.Into(&inserted); err != nil {
			return added, fmt.Errorf("decode merged rows: %w", err)
		}
		for _, item := range inserted {
			var row map[string]any
			if err := json.Unmarshal([]byte(item.Raw), &row); err != nil {
				return added, fmt.Errorf("decode merged row: %w", err)
			}
			added = append(added, row)
		}
	}
	return added, nil
}

// mergeRecords turns mapped rows into recordset rows, keeping the last row of each identity (a
// statement cannot see the rows it inserts) and dropping the rows missing a required value.
func mergeRecords(table mergeTable, rows []map[string]any) []map[string]any {
	records := make([]map[string]any, 0, len(rows))
	positions := make(map[string]int, len(rows))
	for _, row := range rows {
		record := map[string]any{"raw": row}
		complete := true
		for _, column := range table.columns {
			value := column.value(row)
			if value == nil && column.required {
				complete = false
				break
			}
			record[column.name] = value
		}
		if !complete {
			continue
		}
		identity := make([]any, 0, len(table.identity))
		for _, name := range table.identity {
			identity = append(identity, record[name])
		}
		key := fmt.Sprintf("%#v", identity)
		if position, seen := positions[key]; seen {
			records[position] = record
			continue
		}
		positions[key] = len(records)
		records = append(records, record)
	}
	return records
}

// mergeRecordset is the FROM item exposing the JSON rows bound as $2.
func mergeRecordset(table mergeTable) string {
	columns := make([]string, 0, len(table.columns)+1)
	for _, column := range table.columns {
		columns = append(columns, fmt.Sprintf(`"%s" %s`, column.name, column.sqlType))
	}
	columns = append(columns, `"raw" jsonb`)
	return `jsonb_to_recordset($2::jsonb) AS r(` + strings.Join(columns, ", ") + `)`
}

// mergeMatch is the condition telling that stored row s is incoming row r. Times match within a
// millisecond: the provider's microseconds may have been rounded instead of truncated by an
// earlier replace ingestion. The range form keeps the (sessionId, driverNumber, dateUtc) index.
func mergeMatch(table mergeTable) string {
	conditions := []string{`s."sessionId" = $1::text`}
	for _, name := range table.identity {
		switch {
		case name == "dateUtc":
			conditions = append(conditions, `s."dateUtc" BETWEEN r."dateUtc" - interval '1 millisecond' AND r."dateUtc" + interval '1 millisecond'`)
		case table.isRequired(name):
			conditions = append(conditions, fmt.Sprintf(`s."%[1]s" = r."%[1]s"`, name))
		default:
			conditions = append(conditions, fmt.Sprintf(`s."%[1]s" IS NOT DISTINCT FROM r."%[1]s"`, name))
		}
	}
	return strings.Join(conditions, " AND ")
}

// mergeInsertQuery inserts the incoming rows not stored yet, returning them for a reported table.
func mergeInsertQuery(table mergeTable) string {
	names := make([]string, 0, len(table.columns)+1)
	values := make([]string, 0, len(table.columns)+1)
	for _, column := range table.columns {
		names = append(names, fmt.Sprintf(`"%s"`, column.name))
		values = append(values, fmt.Sprintf(`r."%s"`, column.name))
	}
	names = append(names, `"raw"`)
	values = append(values, `r."raw"`)
	query := fmt.Sprintf(`INSERT INTO "%[1]s" AS t ("sessionId", %[2]s) SELECT $1::text, %[3]s FROM %[4]s WHERE NOT EXISTS (SELECT 1 FROM "%[1]s" s WHERE %[5]s)`,
		table.table, strings.Join(names, ", "), strings.Join(values, ", "), mergeRecordset(table), mergeMatch(table))
	if table.report {
		query += ` RETURNING t."raw"::text AS "raw"`
	}
	return query
}

// mergeDeleteQuery removes the stored rows the incoming ones replace.
func mergeDeleteQuery(table mergeTable) string {
	return fmt.Sprintf(`DELETE FROM "%s" s USING %s WHERE %s`, table.table, mergeRecordset(table), mergeMatch(table))
}

// isRequired reports whether the named column never holds NULL in a merged row.
func (t mergeTable) isRequired(name string) bool {
	for _, column := range t.columns {
		if column.name == name {
			return column.required
		}
	}
	return false
}

// mergeDriver reads the driver number.
func mergeDriver(required bool) mergeColumn {
	column := mergeInt("driverNumber", "driver_number")
	column.required = required
	return column
}

// mergeTime reads the row's timestamp, always required, as dateUtc.
func mergeTime(key string) mergeColumn {
	return mergeColumn{name: "dateUtc", sqlType: "timestamp", required: true, value: func(row map[string]any) any {
		parsed, ok := parseTime(row[key])
		if !ok {
			return nil
		}
		return time.Time(parsed).UTC().Truncate(time.Millisecond).Format(mergeTimeLayout)
	}}
}

// mergeInt reads an integer column, with the same rules as a replace ingestion (zero is NULL).
func mergeInt(name string, key string) mergeColumn {
	return mergeColumn{name: name, sqlType: "int", value: func(row map[string]any) any {
		if value := intPtr(row[key]); value != nil {
			return *value
		}
		return nil
	}}
}

// mergeFloat reads a decimal column.
func mergeFloat(name string, key string) mergeColumn {
	return mergeColumn{name: name, sqlType: "double precision", value: func(row map[string]any) any {
		if value := floatPtr(row[key]); value != nil {
			return *value
		}
		return nil
	}}
}

// mergeText reads a text column with the same helper as a replace ingestion (stringPtr: decimals
// kept), cut to maxRunes (0: unbounded): one oversized value must not fail every later batch of a
// live session.
func mergeText(name string, key string, maxRunes int) mergeColumn {
	return mergeColumn{name: name, sqlType: "text", value: func(row map[string]any) any {
		value := stringPtr(row[key])
		if value == nil {
			return nil
		}
		if runes := []rune(*value); maxRunes > 0 && len(runes) > maxRunes {
			return string(runes[:maxRunes])
		}
		return *value
	}}
}
