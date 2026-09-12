/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion.repository.go - Package ports source file for services/championship-service/src/core/ports.
##
*/

package ports

import (
	"context"

	contracts "overdrive/shared/contracts/ingestion"
)

type ChampionshipIngestionRepository interface {
	StoreBatch(ctx context.Context, batch contracts.Batch) error
}
