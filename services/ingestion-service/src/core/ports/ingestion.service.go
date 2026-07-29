/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.service.go - Package ports source file for services/ingestion-service/src/core/ports.
	##
*/

package ports

import (
	"context"

	"overdrive/services/ingestion-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

type OpenF1Provider interface {
	Fetch(ctx context.Context, resource string, request domain.OpenF1IngestionRequest) ([]map[string]any, error)
}

type ProviderMapper interface {
	Map(resource string, rows []map[string]any) (contracts.Dataset, error)
	SupportedResources() []string
}

type BatchDispatcher interface {
	Dispatch(ctx context.Context, service string, batch contracts.Batch) (contracts.DispatchAck, error)
}

type OpenF1IngestionUseCase interface {
	Execute(ctx context.Context, request domain.OpenF1IngestionRequest) (domain.OpenF1IngestionResult, error)
}
