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
	"errors"
	"fmt"
	"log"
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

// ingestionRun carries the state of one Execute call.
type ingestionRun struct {
	request    domain.OpenF1IngestionRequest
	dispatch   bool
	batchID    string
	startedAt  time.Time
	results    []domain.ResourceIngestion
	dispatches map[string]*domain.ServiceDispatch
	// sessionMissing is set when the sessions resource came back empty: OpenF1 does not know
	// the session, so nothing is dispatched.
	sessionMissing bool
}

// Execute ingests the requested OpenF1 resources (all supported ones by default). A failing
// resource is reported in the result without stopping the others. Light resources are batched
// per target service and dispatched first (championship before race-data, which needs the
// session); heavy resources (domain.HeavyResources) of a whole-session request are fetched,
// mapped and dispatched one driver at a time, so neither memory nor a batch ever holds a whole
// session. It returns domain.ErrIngestionFailed when every resource failed, and
// domain.ErrNoOpenF1Data when the sessions resource or every resource came back empty (unknown
// meeting or session), in which case nothing is dispatched.
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
	if err := u.checkSupported(resources); err != nil {
		return domain.OpenF1IngestionResult{}, err
	}

	run := &ingestionRun{
		request:    request,
		dispatch:   request.Dispatch == nil || *request.Dispatch,
		startedAt:  u.now(),
		dispatches: map[string]*domain.ServiceDispatch{},
	}
	run.batchID = fmt.Sprintf("openf1-%d", run.startedAt.UnixNano())

	var light, heavy []string
	for _, resource := range resources {
		if request.DriverNumber == nil && domain.HeavyResources[resource] {
			heavy = append(heavy, resource)
		} else {
			light = append(light, resource)
		}
	}

	u.ingestLight(ctx, run, light)
	if run.sessionMissing {
		return domain.OpenF1IngestionResult{}, fmt.Errorf("%w: session %d", domain.ErrNoOpenF1Data, request.SessionKey)
	}
	u.ingestHeavy(ctx, run, heavy)
	if err := ctx.Err(); err != nil {
		return domain.OpenF1IngestionResult{}, err
	}

	if !hasRows(run.results) && !hasFailure(run.results) {
		return domain.OpenF1IngestionResult{}, domain.ErrNoOpenF1Data
	}
	status, failure := runStatus(run.results)
	if status == "" {
		return domain.OpenF1IngestionResult{}, fmt.Errorf("%w: first failure: %s", domain.ErrIngestionFailed, failure)
	}

	services := make([]string, 0, len(run.dispatches))
	for service := range run.dispatches {
		services = append(services, service)
	}
	sort.Strings(services)
	dispatchResults := make([]domain.ServiceDispatch, 0, len(services))
	for _, service := range services {
		dispatchResults = append(dispatchResults, *run.dispatches[service])
	}

	return domain.OpenF1IngestionResult{
		Status:          status,
		Provider:        "openf1",
		BatchID:         run.batchID,
		MeetingKey:      request.MeetingKey,
		SessionKey:      request.SessionKey,
		DriverNumber:    request.DriverNumber,
		Resources:       run.results,
		Dispatches:      dispatchResults,
		DispatchEnabled: run.dispatch,
		TriggeredAtUTC:  run.startedAt,
		CompletedAtUTC:  u.now(),
	}, nil
}

// checkSupported refuses an unknown resource name before any OpenF1 call (400).
func (u *IngestOpenF1UseCase) checkSupported(resources []string) error {
	supported := map[string]bool{}
	for _, resource := range u.mapper.SupportedResources() {
		supported[resource] = true
	}
	for _, resource := range resources {
		if !supported[resource] {
			return fmt.Errorf("%w %q", domain.ErrUnsupportedResource, resource)
		}
	}
	return nil
}

// ingestLight fetches and maps each light resource, then dispatches one batch per target service
// in service name order. A failure marks only the resources it concerns.
func (u *IngestOpenF1UseCase) ingestLight(ctx context.Context, run *ingestionRun, resources []string) {
	batches := map[string]contracts.Batch{}
	for _, resource := range resources {
		if ctx.Err() != nil {
			return
		}
		rows, err := u.provider.Fetch(ctx, resource, run.request)
		if err != nil {
			run.results = append(run.results, failedResource(resource, fetchFailure(err), err))
			continue
		}
		dataset, err := u.mapper.Map(resource, rows)
		if err != nil {
			run.results = append(run.results, failedResource(resource, "mapping failed", err))
			continue
		}
		if len(dataset.Rows) == 0 {
			if resource == "sessions" {
				// OpenF1 answers an unknown session with "No results found": stop before any
				// dispatch rather than store a meeting or catalog rows for a session that
				// does not exist.
				run.sessionMissing = true
				return
			}
			// Nothing to store. Not dispatched: several datasets replace the session's rows,
			// so an empty batch would wipe what an earlier ingestion stored.
			run.results = append(run.results, domain.ResourceIngestion{
				Resource:      resource,
				DatasetName:   dataset.Name,
				TargetService: dataset.TargetService,
				Status:        domain.ResourceStatusOK,
			})
			continue
		}
		batch, ok := batches[dataset.TargetService]
		if !ok {
			batch = run.newBatch(run.batchID, run.request.DriverNumber)
		}
		batch.Datasets = append(batch.Datasets, dataset)
		batches[dataset.TargetService] = batch
		run.results = append(run.results, domain.ResourceIngestion{
			Resource:      resource,
			DatasetName:   dataset.Name,
			TargetService: dataset.TargetService,
			RowCount:      dataset.RowCount,
			Status:        domain.ResourceStatusOK,
		})
	}
	if !run.dispatch {
		return
	}

	services := make([]string, 0, len(batches))
	for service := range batches {
		services = append(services, service)
	}
	sort.Strings(services)
	for _, service := range services {
		if ctx.Err() != nil {
			return
		}
		ack, err := u.dispatcher.Dispatch(ctx, service, batches[service])
		for index := range run.results {
			result := &run.results[index]
			// Only the resources actually in the batch: an empty one was not sent.
			if result.TargetService != service || result.Status != domain.ResourceStatusOK || result.RowCount == 0 {
				continue
			}
			if err != nil {
				result.Status = domain.ResourceStatusFailed
				result.Error = "dispatch to " + service + " failed"
				continue
			}
			result.Dispatched = true
		}
		if err != nil {
			log.Printf("ingestion %s: dispatch to %s failed: %v", run.batchID, service, err)
			continue
		}
		run.recordDispatch(service, ack)
	}
}

// ingestHeavy fetches, maps and dispatches each heavy resource driver by driver.
func (u *IngestOpenF1UseCase) ingestHeavy(ctx context.Context, run *ingestionRun, resources []string) {
	if len(resources) == 0 || ctx.Err() != nil {
		return
	}
	drivers, err := u.provider.ListDriverNumbers(ctx, run.request)
	if err != nil {
		for _, resource := range resources {
			run.results = append(run.results, failedResource(resource, "driver list unavailable: "+fetchFailure(err), err))
		}
		return
	}

	for _, resource := range resources {
		result := domain.ResourceIngestion{Resource: resource, Status: domain.ResourceStatusOK}
		for _, driver := range drivers {
			if ctx.Err() != nil {
				return
			}
			if err := u.ingestDriver(ctx, run, resource, driver, &result); err != nil {
				log.Printf("ingestion %s: %s for driver %d failed: %v", run.batchID, resource, driver, err)
				result.FailedDrivers = append(result.FailedDrivers, driver)
			}
		}
		switch {
		case len(result.FailedDrivers) == 0:
		case len(result.FailedDrivers) == len(drivers):
			result.Status = domain.ResourceStatusFailed
			result.Error = "every driver failed"
		default:
			result.Status = domain.ResourceStatusPartial
			result.Error = fmt.Sprintf("%d of %d drivers failed", len(result.FailedDrivers), len(drivers))
		}
		run.results = append(run.results, result)
	}
}

// ingestDriver fetches, maps and (when enabled) dispatches one driver's rows of a heavy resource,
// adding them to result.
func (u *IngestOpenF1UseCase) ingestDriver(ctx context.Context, run *ingestionRun, resource string, driver int, result *domain.ResourceIngestion) error {
	request := run.request
	request.DriverNumber = &driver
	rows, err := u.provider.Fetch(ctx, resource, request)
	if err != nil {
		return err
	}
	dataset, err := u.mapper.Map(resource, rows)
	if err != nil {
		return err
	}
	result.DatasetName = dataset.Name
	result.TargetService = dataset.TargetService
	if run.dispatch && len(dataset.Rows) > 0 {
		batch := run.newBatch(fmt.Sprintf("%s-%s-%d", run.batchID, resource, driver), &driver)
		batch.Datasets = []contracts.Dataset{dataset}
		ack, err := u.dispatcher.Dispatch(ctx, dataset.TargetService, batch)
		if err != nil {
			return err
		}
		run.recordDispatch(dataset.TargetService, ack)
		result.Dispatched = true
	}
	result.RowCount += dataset.RowCount
	return nil
}

// newBatch builds an empty batch for this run.
func (run *ingestionRun) newBatch(batchID string, driverNumber *int) contracts.Batch {
	return contracts.Batch{
		BatchID:     batchID,
		Provider:    "openf1",
		TriggeredAt: run.startedAt,
		Context: contracts.Context{
			MeetingKey:   run.request.MeetingKey,
			SessionKey:   run.request.SessionKey,
			DriverNumber: driverNumber,
			Provider:     "openf1",
		},
		Metadata: map[string]any{
			"source": "ingestion-service",
		},
	}
}

// recordDispatch adds an acknowledgement to the per-service totals.
func (run *ingestionRun) recordDispatch(service string, ack contracts.DispatchAck) {
	total, ok := run.dispatches[service]
	if !ok {
		name := ack.Service
		if name == "" {
			name = service
		}
		total = &domain.ServiceDispatch{Service: name}
		run.dispatches[service] = total
	}
	total.DatasetCount += ack.DatasetCount
	total.RowCount += ack.TotalRowCount
}

// failedResource builds the result of a resource that could not be ingested; the technical
// cause is logged, the result only carries a short reason.
func failedResource(resource string, reason string, err error) domain.ResourceIngestion {
	log.Printf("ingestion: resource %s failed: %v", resource, err)
	return domain.ResourceIngestion{Resource: resource, Status: domain.ResourceStatusFailed, Error: reason}
}

// fetchFailure turns an OpenF1 fetch error into a short reason for the result.
func fetchFailure(err error) string {
	switch {
	case errors.Is(err, domain.ErrResponseTooLarge):
		return "OpenF1 response too large"
	case errors.Is(err, context.DeadlineExceeded):
		return "OpenF1 request timed out"
	default:
		return "OpenF1 request failed"
	}
}

// hasRows reports whether any resource brought at least one row.
func hasRows(results []domain.ResourceIngestion) bool {
	for _, result := range results {
		if result.RowCount > 0 {
			return true
		}
	}
	return false
}

// hasFailure reports whether any resource is not ok.
func hasFailure(results []domain.ResourceIngestion) bool {
	for _, result := range results {
		if result.Status != domain.ResourceStatusOK {
			return true
		}
	}
	return false
}

// runStatus returns completed when every resource is ok, partial when at least one is ok or
// partial, and "" (with the first failure reason) when every resource failed.
func runStatus(results []domain.ResourceIngestion) (string, string) {
	usable, failure := 0, ""
	for _, result := range results {
		if result.Status != domain.ResourceStatusFailed {
			usable++
		} else if failure == "" {
			failure = result.Resource + ": " + result.Error
		}
	}
	switch {
	case usable == 0:
		return "", failure
	case usable == len(results):
		for _, result := range results {
			if result.Status != domain.ResourceStatusOK {
				return domain.IngestionStatusPartial, ""
			}
		}
		return domain.IngestionStatusCompleted, ""
	default:
		return domain.IngestionStatusPartial, ""
	}
}
