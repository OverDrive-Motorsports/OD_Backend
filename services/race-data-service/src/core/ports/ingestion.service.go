/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.service.go - Package ports source file for services/race-data-service/src/core/ports.
	##
*/

package ports

import (
	"context"

	"overdrive/services/race-data-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

type IngestionBatchUseCase interface {
	Execute(ctx context.Context, batch contracts.Batch) (domain.IngestionBatchAck, error)
}
