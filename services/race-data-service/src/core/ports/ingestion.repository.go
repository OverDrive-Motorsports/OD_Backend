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
	StoreBatch(ctx context.Context, batch contracts.Batch) error
}
