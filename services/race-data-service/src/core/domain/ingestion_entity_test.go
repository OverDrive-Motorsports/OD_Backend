/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_entity_test.go - Package domain source file for services/race-data-service/src/core/domain.
##
*/

package domain

import (
	"testing"
	"time"

	contracts "overdrive/shared/contracts/ingestion"
)

// TestNewIngestionBatchAck proves the acknowledgement sums row counts across every dataset in the
// batch and carries through the service name, batch ID, and the supplied timestamp unmodified.
func TestNewIngestionBatchAck(t *testing.T) {
	now := time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC)
	batch := contracts.Batch{
		BatchID: "b1",
		Datasets: []contracts.Dataset{
			{Name: "laps", RowCount: 3},
			{Name: "telemetry", RowCount: 7},
		},
	}

	ack := NewIngestionBatchAck("race-data-service", batch, now)

	if !ack.Accepted {
		t.Fatal("expected Accepted=true")
	}
	if ack.Service != "race-data-service" || ack.BatchID != "b1" {
		t.Fatalf("unexpected service/batch id: %+v", ack)
	}
	if ack.DatasetCount != 2 {
		t.Fatalf("DatasetCount = %d, want 2", ack.DatasetCount)
	}
	if ack.TotalRowCount != 10 {
		t.Fatalf("TotalRowCount = %d, want 10", ack.TotalRowCount)
	}
	if !ack.ProcessedAtUtc.Equal(now) {
		t.Fatalf("ProcessedAtUtc = %v, want %v", ack.ProcessedAtUtc, now)
	}
}

// TestNewIngestionBatchAck_NoDatasets proves an empty dataset list produces a zero row/dataset
// count rather than panicking.
func TestNewIngestionBatchAck_NoDatasets(t *testing.T) {
	ack := NewIngestionBatchAck("race-data-service", contracts.Batch{BatchID: "b2"}, time.Now())
	if ack.DatasetCount != 0 || ack.TotalRowCount != 0 {
		t.Fatalf("expected zero counts, got %+v", ack)
	}
}
