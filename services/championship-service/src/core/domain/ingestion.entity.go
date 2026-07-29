/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.entity.go - Package domain source file for services/championship-service/src/core/domain.
	##
*/

package domain

import (
	"time"

	contracts "overdrive/shared/contracts/ingestion"
)

type IngestionBatchAck struct {
	Service        string    `json:"service"`
	BatchID        string    `json:"batch_id"`
	Accepted       bool      `json:"accepted"`
	DatasetCount   int       `json:"dataset_count"`
	TotalRowCount  int       `json:"total_row_count"`
	ProcessedAtUtc time.Time `json:"processed_at_utc"`
}

// NewIngestionBatchAck builds and returns a ingestion batch ack with its required dependencies.
func NewIngestionBatchAck(serviceName string, batch contracts.Batch, now time.Time) IngestionBatchAck {
	rows := 0
	for _, dataset := range batch.Datasets {
		rows += dataset.RowCount
	}
	return IngestionBatchAck{
		Service:        serviceName,
		BatchID:        batch.BatchID,
		Accepted:       true,
		DatasetCount:   len(batch.Datasets),
		TotalRowCount:  rows,
		ProcessedAtUtc: now,
	}
}
