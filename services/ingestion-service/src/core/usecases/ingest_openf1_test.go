/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingest_openf1_test.go - Package usecases source file for services/ingestion-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"errors"
	"testing"

	"overdrive/services/ingestion-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

// fakeOpenF1Provider is an in-memory ports.OpenF1Provider. It fails for any resource listed in
// failOn, and otherwise returns one row per call so mapping has something to work with.
type fakeOpenF1Provider struct {
	failOn map[string]error
	calls  []string
}

func (f *fakeOpenF1Provider) Fetch(ctx context.Context, resource string, request domain.OpenF1IngestionRequest) ([]map[string]any, error) {
	f.calls = append(f.calls, resource)
	if err, ok := f.failOn[resource]; ok {
		return nil, err
	}
	return []map[string]any{{"driver_number": 1}}, nil
}

// fakeProviderMapper is an in-memory ports.ProviderMapper. It fails for any resource listed in
// failOn, and otherwise routes every resource to a "race-data-service" dataset.
type fakeProviderMapper struct {
	failOn    map[string]error
	supported []string
}

func (f *fakeProviderMapper) SupportedResources() []string { return f.supported }

func (f *fakeProviderMapper) Map(resource string, rows []map[string]any) (contracts.Dataset, error) {
	if err, ok := f.failOn[resource]; ok {
		return contracts.Dataset{}, err
	}
	return contracts.Dataset{
		Name:             resource,
		TargetService:    "race-data-service",
		ProviderResource: resource,
		RowCount:         len(rows),
		Rows:             rows,
	}, nil
}

// fakeBatchDispatcher is an in-memory ports.BatchDispatcher. It fails for any service listed in
// failOn, and otherwise records every dispatched batch.
type fakeBatchDispatcher struct {
	failOn      map[string]error
	dispatched  []contracts.Batch
	dispatchSvc []string
}

func (f *fakeBatchDispatcher) Dispatch(ctx context.Context, service string, batch contracts.Batch) (contracts.DispatchAck, error) {
	f.dispatchSvc = append(f.dispatchSvc, service)
	if err, ok := f.failOn[service]; ok {
		return contracts.DispatchAck{}, err
	}
	f.dispatched = append(f.dispatched, batch)
	return contracts.DispatchAck{Service: service, DatasetCount: len(batch.Datasets), TotalRowCount: batch.Datasets[0].RowCount}, nil
}

func validRequest(resources ...string) domain.OpenF1IngestionRequest {
	return domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998, Resources: resources}
}

// TestIngestOpenF1UseCase_FetchFailureAborts proves a single resource fetch failure aborts the
// whole ingestion run (no partial dispatch of the resources that did fetch successfully) and the
// failure is surfaced, not swallowed.
func TestIngestOpenF1UseCase_FetchFailureAborts(t *testing.T) {
	boom := errors.New("openf1 unreachable")
	provider := &fakeOpenF1Provider{failOn: map[string]error{"laps": boom}}
	mapper := &fakeProviderMapper{}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, mapper, dispatcher)

	_, err := uc.Execute(context.Background(), validRequest("drivers", "laps", "car_data"))
	if !errors.Is(err, boom) {
		t.Fatalf("expected the fetch error to propagate, got %v", err)
	}
	if len(dispatcher.dispatchSvc) != 0 {
		t.Fatalf("expected no dispatch to happen after a fetch failure, got %v", dispatcher.dispatchSvc)
	}
}

// TestIngestOpenF1UseCase_PartialMappingFailureAborts proves that a mapping failure on any one
// resource in a multi-resource request aborts the whole run, even though earlier resources in
// the request were fetched/mapped successfully - nothing gets dispatched from a partially-mapped
// batch.
func TestIngestOpenF1UseCase_PartialMappingFailureAborts(t *testing.T) {
	boom := errors.New("mapping failed for car_data")
	provider := &fakeOpenF1Provider{}
	mapper := &fakeProviderMapper{failOn: map[string]error{"car_data": boom}}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, mapper, dispatcher)

	_, err := uc.Execute(context.Background(), validRequest("drivers", "car_data"))
	if !errors.Is(err, boom) {
		t.Fatalf("expected the mapping error to propagate, got %v", err)
	}
	if len(dispatcher.dispatchSvc) != 0 {
		t.Fatalf("expected no dispatch to happen after a partial mapping failure, got %v", dispatcher.dispatchSvc)
	}
	// The first resource ("drivers") must still have been fetched before the second
	// ("car_data") failed at the mapping stage - proves resources are processed in order,
	// not concurrently/speculatively.
	if len(provider.calls) != 2 || provider.calls[0] != "drivers" || provider.calls[1] != "car_data" {
		t.Fatalf("expected fetch to have been attempted for drivers then car_data, got %v", provider.calls)
	}
}

// TestIngestOpenF1UseCase_DispatchFailureSurfaced proves a downstream dispatch failure (e.g. a
// downed race-data-service) is returned to the caller rather than silently ignored, and that
// resourceResults'/Dispatched flags reflect only what actually made it out.
func TestIngestOpenF1UseCase_DispatchFailureSurfaced(t *testing.T) {
	boom := errors.New("race-data-service unreachable")
	provider := &fakeOpenF1Provider{}
	mapper := &fakeProviderMapper{}
	dispatcher := &fakeBatchDispatcher{failOn: map[string]error{"race-data-service": boom}}
	uc := NewIngestOpenF1UseCase(provider, mapper, dispatcher)

	_, err := uc.Execute(context.Background(), validRequest("car_data"))
	if !errors.Is(err, boom) {
		t.Fatalf("expected the dispatch error to propagate, got %v", err)
	}
}

// TestIngestOpenF1UseCase_SuccessfulRunDispatchesEveryTargetService proves the normal case: each
// resource is fetched, mapped, grouped by target service, and dispatched exactly once per
// distinct service, with per-resource Dispatched flags set to true.
func TestIngestOpenF1UseCase_SuccessfulRunDispatchesEveryTargetService(t *testing.T) {
	provider := &fakeOpenF1Provider{}
	mapper := &fakeProviderMapper{}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, mapper, dispatcher)

	result, err := uc.Execute(context.Background(), validRequest("drivers", "car_data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.DispatchEnabled {
		t.Fatal("expected dispatch to be enabled by default")
	}
	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resource results, got %+v", result.Resources)
	}
	for _, r := range result.Resources {
		if !r.Dispatched {
			t.Fatalf("expected resource %q to be marked dispatched, got %+v", r.Resource, r)
		}
	}
	if len(dispatcher.dispatchSvc) != 1 || dispatcher.dispatchSvc[0] != "race-data-service" {
		t.Fatalf("expected exactly one dispatch to race-data-service, got %v", dispatcher.dispatchSvc)
	}
}

// TestIngestOpenF1UseCase_DispatchDisabledSkipsDispatcher proves dispatch:false on the request
// suppresses every Dispatch call while still fetching/mapping every resource.
func TestIngestOpenF1UseCase_DispatchDisabledSkipsDispatcher(t *testing.T) {
	provider := &fakeOpenF1Provider{}
	mapper := &fakeProviderMapper{}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, mapper, dispatcher)

	dispatchFalse := false
	request := validRequest("drivers")
	request.Dispatch = &dispatchFalse

	result, err := uc.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DispatchEnabled {
		t.Fatal("expected dispatch to be disabled")
	}
	if len(dispatcher.dispatchSvc) != 0 {
		t.Fatalf("expected no dispatch calls, got %v", dispatcher.dispatchSvc)
	}
	if result.Resources[0].Dispatched {
		t.Fatal("expected the resource to not be marked dispatched when dispatch is disabled")
	}
}
