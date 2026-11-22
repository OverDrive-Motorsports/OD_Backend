/**
##
## OverDrive 2026
## All Technical rights reserved
##
## batch_test.go - Unit tests pinning the JSON shape of the ingestion batch contract.
##
*/

package ingestion

import (
	"encoding/json"
	"testing"
	"time"
)

// TestBatchJSONFieldNames pins the snake_case wire names of a batch. ingestion-service writes
// them and championship-service and race-data-service decode them with their own copy of the
// field names, so a renamed tag drops data on the receiving side without any error.
func TestBatchJSONFieldNames(t *testing.T) {
	driver := 63
	batch := Batch{
		BatchID:     "b1",
		Provider:    "openf1",
		TriggeredAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Context:     Context{MeetingKey: 1229, SessionKey: 9998, DriverNumber: &driver, Provider: "openf1"},
		Datasets: []Dataset{{
			Name: "laps", Category: "race", TargetService: "race-data", ProviderResource: "laps",
			SchemaVersion: "v1", RowCount: 1, Rows: []map[string]any{{"lap_number": 1}},
			Metadata: map[string]any{"k": "v"},
		}},
		Metadata: map[string]any{"source": "test"},
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"batch_id":"b1","provider":"openf1","triggered_at":"2026-01-02T03:04:05Z",` +
		`"context":{"meeting_key":1229,"session_key":9998,"driver_number":63,"provider":"openf1"},` +
		`"datasets":[{"name":"laps","category":"race","target_service":"race-data","provider_resource":"laps",` +
		`"schema_version":"v1","row_count":1,"rows":[{"lap_number":1}],"metadata":{"k":"v"}}],` +
		`"metadata":{"source":"test"}}`
	if string(raw) != want {
		t.Fatalf("batch JSON =\n%s\nwant\n%s", raw, want)
	}
}

// TestBatchJSONOmitsOptionalFields proves the optional keys (context keys, metadata) are left
// out when empty, while the provider is always written.
func TestBatchJSONOmitsOptionalFields(t *testing.T) {
	raw, err := json.Marshal(Context{Provider: "openf1"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"provider":"openf1"}` {
		t.Fatalf("context JSON = %s", raw)
	}
	raw, err = json.Marshal(Dataset{Name: "laps"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"name":"laps","category":"","target_service":"","provider_resource":"","schema_version":"","row_count":0,"rows":null}`
	if string(raw) != want {
		t.Fatalf("dataset JSON = %s, want %s", raw, want)
	}
}

// TestDispatchAckJSONFieldNames pins the acknowledgement the receiving services send back to
// ingestion-service.
func TestDispatchAckJSONFieldNames(t *testing.T) {
	ack := DispatchAck{
		Service: "race-data", BatchID: "b1", Accepted: true, DatasetCount: 2, TotalRowCount: 10,
		ProcessedAtUtc: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	raw, err := json.Marshal(ack)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"service":"race-data","batch_id":"b1","accepted":true,"dataset_count":2,"total_row_count":10,"processed_at_utc":"2026-01-02T03:04:05Z"}`
	if string(raw) != want {
		t.Fatalf("ack JSON = %s, want %s", raw, want)
	}
}
