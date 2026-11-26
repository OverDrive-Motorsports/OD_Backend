/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_merge_repository_test.go - Unit tests for the live batch merge queries and row mapping.
##
*/

package prismaadapter

import (
	"strings"
	"testing"
)

// TestMergeInsertQuery pins the merge SQL of a sample table: every value is bound, the session is
// part of the match, and times match within a millisecond.
func TestMergeInsertQuery(t *testing.T) {
	want := `INSERT INTO "RacePositionSample" AS t ("sessionId", "driverNumber", "dateUtc", "lapNumber", "position", "raw")` +
		` SELECT $1::text, r."driverNumber", r."dateUtc", r."lapNumber", r."position", r."raw"` +
		` FROM jsonb_to_recordset($2::jsonb) AS r("driverNumber" int, "dateUtc" timestamp, "lapNumber" int, "position" int, "raw" jsonb)` +
		` WHERE NOT EXISTS (SELECT 1 FROM "RacePositionSample" s WHERE s."sessionId" = $1::text AND s."driverNumber" = r."driverNumber"` +
		` AND s."dateUtc" BETWEEN r."dateUtc" - interval '1 millisecond' AND r."dateUtc" + interval '1 millisecond')`
	if got := mergeInsertQuery(mergeTables["position"]); got != want {
		t.Fatalf("query =\n%s\nwant\n%s", got, want)
	}
}

// TestMergeQueries_Variants proves a nullable identity is compared NULL-safe, race control reports
// its inserted rows, and a changing dataset deletes only its own session's matching rows.
func TestMergeQueries_Variants(t *testing.T) {
	raceControl := mergeInsertQuery(mergeTables["race_control"])
	for _, part := range []string{`s."message" IS NOT DISTINCT FROM r."message"`, `s."driverNumber" IS NOT DISTINCT FROM r."driverNumber"`, `RETURNING t."raw"::text AS "raw"`} {
		if !strings.Contains(raceControl, part) {
			t.Fatalf("race control query misses %q: %s", part, raceControl)
		}
	}
	if strings.Contains(mergeInsertQuery(mergeTables["telemetry"]), "RETURNING") {
		t.Fatal("telemetry rows must not be returned")
	}
	remove := mergeDeleteQuery(mergeTables["stints"])
	if !strings.HasPrefix(remove, `DELETE FROM "RaceStint" s USING jsonb_to_recordset($2::jsonb)`) ||
		!strings.HasSuffix(remove, `WHERE s."sessionId" = $1::text AND s."driverNumber" = r."driverNumber" AND s."stintNumber" IS NOT DISTINCT FROM r."stintNumber"`) {
		t.Fatalf("stints delete = %s", remove)
	}
}

// TestMergeRecords proves rows without a driver or a valid time are dropped, times are sent in UTC
// truncated to the millisecond, an identity repeated in a batch keeps its last row, two driver
// numbers are never confused, and a gap keeps its decimals.
func TestMergeRecords(t *testing.T) {
	records := mergeRecords(mergeTables["position"], []map[string]any{
		{"driver_number": float64(44), "sample_time_utc": "2026-03-08T16:00:00.123987+01:00", "position": float64(3)},
		{"driver_number": float64(44), "sample_time_utc": "2026-03-08T15:00:00.123100Z", "position": float64(2)},
		{"driver_number": float64(63), "sample_time_utc": "2026-03-08T15:00:00.123Z", "position": float64(1)},
		{"driver_number": float64(63), "sample_time_utc": "not a time"},
		{"sample_time_utc": "2026-03-08T15:00:00Z"},
	})
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	if first := records[0]; first["dateUtc"] != "2026-03-08T15:00:00.123" || first["position"] != 2 || first["lapNumber"] != nil || records[1]["driverNumber"] != 63 {
		t.Fatalf("records = %+v", records)
	}
	gaps := mergeRecords(mergeTables["intervals"], []map[string]any{{"driver_number": float64(1), "sample_time_utc": "2026-03-08T15:00:00Z", "gap_to_leader": 12.345, "interval_to_front_driver": "+1 LAP"}})
	if gaps[0]["gapToLeader"] != "12.345" || gaps[0]["intervalToFrontDriver"] != "+1 LAP" {
		t.Fatalf("gaps = %+v", gaps)
	}
}

// TestMergeText proves an oversized value is cut to the column size, by character, and that an
// absent one stays NULL.
func TestMergeText(t *testing.T) {
	column := mergeText("flag", "flag", 3)
	if got := column.value(map[string]any{"flag": "\u00e9\u00e9\u00e9\u00e9"}); got != "\u00e9\u00e9\u00e9" {
		t.Fatalf("got %v", got)
	}
	if got := column.value(map[string]any{}); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
