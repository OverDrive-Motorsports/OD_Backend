/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_entity_test.go - Package domain source file for services/championship-service/src/core/domain.
##
*/

package domain

import (
	"testing"
	"time"

	contracts "overdrive/shared/contracts/ingestion"
)

// TestNewIngestionBatchAck proves the ack aggregates dataset count and total row count across
// every dataset in the batch, and echoes back the service name/batch id/timestamp it was given.
func TestNewIngestionBatchAck(t *testing.T) {
	batch := contracts.Batch{
		BatchID: "batch-1",
		Datasets: []contracts.Dataset{
			{Name: "event_catalog", RowCount: 2},
			{Name: "session_catalog", RowCount: 5},
		},
	}
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

	got := NewIngestionBatchAck("championship-service", batch, now)

	if got.Service != "championship-service" || got.BatchID != "batch-1" || !got.Accepted {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.DatasetCount != 2 {
		t.Fatalf("DatasetCount = %d, want 2", got.DatasetCount)
	}
	if got.TotalRowCount != 7 {
		t.Fatalf("TotalRowCount = %d, want 7", got.TotalRowCount)
	}
	if !got.ProcessedAtUtc.Equal(now) {
		t.Fatalf("ProcessedAtUtc = %v, want %v", got.ProcessedAtUtc, now)
	}
}

// TestNewIngestionBatchAck_NoDatasets proves an empty dataset list yields zero counts rather
// than erroring.
func TestNewIngestionBatchAck_NoDatasets(t *testing.T) {
	got := NewIngestionBatchAck("championship-service", contracts.Batch{BatchID: "batch-1"}, time.Now())
	if got.DatasetCount != 0 || got.TotalRowCount != 0 {
		t.Fatalf("expected zero counts, got %+v", got)
	}
}
