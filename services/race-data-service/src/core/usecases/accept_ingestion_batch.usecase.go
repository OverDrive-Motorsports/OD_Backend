/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## accept_ingestion_batch.usecase.go - Package usecases source file for services/race-data-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"
	"fmt"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
	contracts "overdrive/shared/contracts/ingestion"
)

type AcceptIngestionBatchUseCase struct {
	serviceName string
	repository  ports.RaceDataIngestionRepository
	now         func() time.Time
}

// NewAcceptIngestionBatchUseCase builds and returns a accept ingestion batch use case with its required dependencies.
func NewAcceptIngestionBatchUseCase(serviceName string, repository ports.RaceDataIngestionRepository) *AcceptIngestionBatchUseCase {
	return &AcceptIngestionBatchUseCase{
		serviceName: serviceName,
		repository:  repository,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// Execute runs the use case workflow and returns the resulting domain payload.
func (u *AcceptIngestionBatchUseCase) Execute(ctx context.Context, batch contracts.Batch) (domain.IngestionBatchAck, error) {
	if batch.BatchID == "" {
		return domain.IngestionBatchAck{}, fmt.Errorf("batch_id is required")
	}
	if len(batch.Datasets) == 0 {
		return domain.IngestionBatchAck{}, fmt.Errorf("at least one dataset is required")
	}
	for _, dataset := range batch.Datasets {
		if dataset.TargetService != u.serviceName {
			return domain.IngestionBatchAck{}, fmt.Errorf("dataset %s targets %s, not %s", dataset.Name, dataset.TargetService, u.serviceName)
		}
	}
	if err := u.repository.StoreBatch(ctx, batch); err != nil {
		return domain.IngestionBatchAck{}, err
	}
	return domain.NewIngestionBatchAck(u.serviceName, batch, u.now()), nil
}
