/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingest_openf1.usecase.go - Package usecases source file for services/ingestion-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"fmt"
	"sort"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
	"overdrive/services/ingestion-service/src/core/ports"
	contracts "overdrive/shared/contracts/ingestion"
)

type IngestOpenF1UseCase struct {
	provider   ports.OpenF1Provider
	mapper     ports.ProviderMapper
	dispatcher ports.BatchDispatcher
	now        func() time.Time
}

// NewIngestOpenF1UseCase builds and returns a ingest open f1 use case with its required dependencies.
func NewIngestOpenF1UseCase(provider ports.OpenF1Provider, mapper ports.ProviderMapper, dispatcher ports.BatchDispatcher) *IngestOpenF1UseCase {
	return &IngestOpenF1UseCase{
		provider:   provider,
		mapper:     mapper,
		dispatcher: dispatcher,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// Execute runs the use case workflow and returns the resulting domain payload.
func (u *IngestOpenF1UseCase) Execute(ctx context.Context, request domain.OpenF1IngestionRequest) (domain.OpenF1IngestionResult, error) {
	if request.MeetingKey <= 0 {
		return domain.OpenF1IngestionResult{}, fmt.Errorf("meeting_key must be greater than 0")
	}
	if request.SessionKey <= 0 {
		return domain.OpenF1IngestionResult{}, fmt.Errorf("session_key must be greater than 0")
	}

	resources := request.Resources
	if len(resources) == 0 {
		resources = u.mapper.SupportedResources()
	}

	dispatchEnabled := true
	if request.Dispatch != nil {
		dispatchEnabled = *request.Dispatch
	}

	startedAt := u.now()
	batchID := fmt.Sprintf("openf1-%d", startedAt.UnixNano())
	batches := map[string]contracts.Batch{}
	resourceResults := make([]domain.ResourceIngestion, 0, len(resources))

	for _, resource := range resources {
		rows, err := u.provider.Fetch(ctx, resource, request)
		if err != nil {
			return domain.OpenF1IngestionResult{}, fmt.Errorf("fetch %s: %w", resource, err)
		}

		dataset, err := u.mapper.Map(resource, rows)
		if err != nil {
			return domain.OpenF1IngestionResult{}, fmt.Errorf("map %s: %w", resource, err)
		}

		service := dataset.TargetService
		batch := batches[service]
		if batch.BatchID == "" {
			batch = contracts.Batch{
				BatchID:     batchID,
				Provider:    "openf1",
				TriggeredAt: startedAt,
				Context: contracts.Context{
					MeetingKey:   request.MeetingKey,
					SessionKey:   request.SessionKey,
					DriverNumber: request.DriverNumber,
					Provider:     "openf1",
				},
				Metadata: map[string]any{
					"source": "ingestion-service",
				},
			}
		}
		batch.Datasets = append(batch.Datasets, dataset)
		batches[service] = batch

		resourceResults = append(resourceResults, domain.ResourceIngestion{
			Resource:      resource,
			DatasetName:   dataset.Name,
			TargetService: service,
			RowCount:      dataset.RowCount,
			Dispatched:    false,
		})
	}

	dispatchResults := make([]domain.ServiceDispatch, 0, len(batches))
	if dispatchEnabled {
		services := make([]string, 0, len(batches))
		for service := range batches {
			services = append(services, service)
		}
		sort.Strings(services)

		for _, service := range services {
			ack, err := u.dispatcher.Dispatch(ctx, service, batches[service])
			if err != nil {
				return domain.OpenF1IngestionResult{}, fmt.Errorf("dispatch %s: %w", service, err)
			}

			for index := range resourceResults {
				if resourceResults[index].TargetService == service {
					resourceResults[index].Dispatched = true
				}
			}

			dispatchResults = append(dispatchResults, domain.ServiceDispatch{
				Service:      ack.Service,
				DatasetCount: ack.DatasetCount,
				RowCount:     ack.TotalRowCount,
			})
		}
	}

	return domain.OpenF1IngestionResult{
		Provider:        "openf1",
		BatchID:         batchID,
		MeetingKey:      request.MeetingKey,
		SessionKey:      request.SessionKey,
		DriverNumber:    request.DriverNumber,
		Resources:       resourceResults,
		Dispatches:      dispatchResults,
		DispatchEnabled: dispatchEnabled,
		TriggeredAtUTC:  startedAt,
		CompletedAtUTC:  u.now(),
	}, nil
}
