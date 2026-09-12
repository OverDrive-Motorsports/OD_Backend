/**
##
## OverDrive 2026
## All Technical rights reserved
##
## mapper_test.go - Package openf1 source file for services/ingestion-service/src/adapters/providers/openf1.
##
*/

package openf1

import (
	"errors"
	"testing"

	"overdrive/services/ingestion-service/src/core/domain"
)

// TestMapper_Map_UnsupportedResource proves an unrecognized resource name returns the
// domain.ErrUnsupportedResource sentinel (rather than a raw string-matched error), since
// ingestion.controller.go classifies it with errors.Is into a 400.
func TestMapper_Map_UnsupportedResource(t *testing.T) {
	m := NewMapper()
	_, err := m.Map("not-a-real-resource", nil)
	if !errors.Is(err, domain.ErrUnsupportedResource) {
		t.Fatalf("expected domain.ErrUnsupportedResource, got %v", err)
	}
}

// TestMapper_Map_EverySupportedResourceIsRoutable proves every resource name returned by
// SupportedResources() is handled by Map's switch (no accidental fallthrough to the unsupported
// branch when a new resource is added to one list but not the other).
func TestMapper_Map_EverySupportedResourceIsRoutable(t *testing.T) {
	m := NewMapper()
	for _, resource := range m.SupportedResources() {
		dataset, err := m.Map(resource, []map[string]any{{"driver_number": 1}})
		if err != nil {
			t.Fatalf("resource %q: unexpected error %v", resource, err)
		}
		if dataset.Name == "" || dataset.TargetService == "" {
			t.Fatalf("resource %q: expected a populated dataset, got %+v", resource, dataset)
		}
	}
}

// TestMapper_MapLaps_FieldRenaming pins the snake_case OpenF1 field -> internal field mapping
// for one representative dataset (laps), including that the original raw row is preserved
// verbatim under "raw" alongside the renamed fields.
func TestMapper_MapLaps_FieldRenaming(t *testing.T) {
	m := NewMapper()
	rows := []map[string]any{{
		"session_key":       9998,
		"meeting_key":       1141,
		"driver_number":     63,
		"lap_number":        5,
		"date_start":        "2026-03-08T15:00:00Z",
		"lap_duration":      91.234,
		"duration_sector_1": 28.1,
		"duration_sector_2": 31.0,
		"duration_sector_3": 32.134,
		"i1_speed":          310,
		"i2_speed":          300,
		"st_speed":          320,
		"is_pit_out_lap":    false,
	}}

	dataset, err := m.Map("laps", rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dataset.Name != "lap_timing" || dataset.TargetService != "race-data-service" || dataset.ProviderResource != "laps" {
		t.Fatalf("unexpected dataset metadata: %+v", dataset)
	}
	if dataset.RowCount != 1 || len(dataset.Rows) != 1 {
		t.Fatalf("expected exactly 1 mapped row, got %+v", dataset.Rows)
	}

	mapped := dataset.Rows[0]
	wantEquals := map[string]any{
		"session_key":      9998,
		"driver_number":    63,
		"lap_number":       5,
		"lap_duration_sec": 91.234,
		"sector_1_sec":     28.1,
		"speed_i1_kph":     310,
	}
	for key, want := range wantEquals {
		if got := mapped[key]; got != want {
			t.Fatalf("mapped[%q] = %v, want %v", key, got, want)
		}
	}
	if mapped["started_at_utc"] != "2026-03-08T15:00:00Z" {
		t.Fatalf("expected started_at_utc to carry through date_start, got %v", mapped["started_at_utc"])
	}
	raw, ok := mapped["raw"].(map[string]any)
	if !ok {
		t.Fatalf("expected raw to be preserved as the original row map, got %T", mapped["raw"])
	}
	if raw["lap_duration"] != 91.234 {
		t.Fatalf("expected raw to preserve the original OpenF1 field name/value, got %v", raw["lap_duration"])
	}
}

// TestMapper_MapTelemetry_CarriesDrsStateThrough proves the raw drs field survives mapping under
// its internal name (drs_state) unmodified - the DRS-active threshold itself is derived
// downstream in race-data-service, not here.
func TestMapper_MapTelemetry_CarriesDrsStateThrough(t *testing.T) {
	m := NewMapper()
	rows := []map[string]any{{"driver_number": 1, "date": "2026-03-08T15:00:00Z", "drs": 12}}

	dataset, err := m.Map("car_data", rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := dataset.Rows[0]["drs_state"]; got != 12 {
		t.Fatalf("expected drs_state=12, got %v", got)
	}
}
