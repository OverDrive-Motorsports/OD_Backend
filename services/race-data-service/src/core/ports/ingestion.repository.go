/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion.repository.go - Package ports source file for services/race-data-service/src/core/ports.
##
*/

package ports

import (
	"context"

	contracts "overdrive/shared/contracts/ingestion"
)

type RaceDataIngestionRepository interface {
	// StoreBatch replaces the stored rows of the batch's scope.
	StoreBatch(ctx context.Context, batch contracts.Batch) error
	// MergeBatch adds the rows of a live batch that are not stored yet, so overlapping batches
	// never duplicate a sample. It returns the race control rows it actually added, also when
	// it fails part way.
	MergeBatch(ctx context.Context, batch contracts.Batch) ([]map[string]any, error)
}
