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
	"fmt"
	"strconv"
	"strings"
	"testing"

	"overdrive/services/ingestion-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

// fakeOpenF1Provider is an in-memory ports.OpenF1Provider. failOn fails a resource ("laps") or a
// resource for one driver ("car_data/44"); every other call returns one row. Calls are recorded
// the same way.
type fakeOpenF1Provider struct {
	failOn     map[string]error
	drivers    []int
	driversErr error
	calls      []string
}

func (f *fakeOpenF1Provider) Fetch(ctx context.Context, resource string, request domain.OpenF1IngestionRequest) ([]map[string]any, error) {
	key := resource
	if request.DriverNumber != nil {
		key = resource + "/" + strconv.Itoa(*request.DriverNumber)
	}
	f.calls = append(f.calls, key)
	if err, ok := f.failOn[key]; ok {
		return nil, err
	}
	if err, ok := f.failOn[resource]; ok {
		return nil, err
	}
	driver := 1
	if request.DriverNumber != nil {
		driver = *request.DriverNumber
	}
	return []map[string]any{{"driver_number": driver}}, nil
}

func (f *fakeOpenF1Provider) ListDriverNumbers(ctx context.Context, request domain.OpenF1IngestionRequest) ([]int, error) {
	f.calls = append(f.calls, "driver-list")
	return f.drivers, f.driversErr
}

// fakeProviderMapper is an in-memory ports.ProviderMapper. It fails for any resource listed in
// failOn; meetings, sessions and drivers go to championship-service, the rest to
// race-data-service.
type fakeProviderMapper struct {
	failOn    map[string]error
	supported []string
}

func (f *fakeProviderMapper) SupportedResources() []string {
	if f.supported != nil {
		return f.supported
	}
	return []string{"meetings", "sessions", "drivers", "laps", "car_data", "location"}
}

func (f *fakeProviderMapper) Map(resource string, rows []map[string]any) (contracts.Dataset, error) {
	if err, ok := f.failOn[resource]; ok {
		return contracts.Dataset{}, err
	}
	target := "race-data-service"
	if resource == "meetings" || resource == "sessions" || resource == "drivers" {
		target = "championship-service"
	}
	return contracts.Dataset{
		Name:             resource,
		TargetService:    target,
		ProviderResource: resource,
		RowCount:         len(rows),
		Rows:             rows,
	}, nil
}

// fakeBatchDispatcher is an in-memory ports.BatchDispatcher. It fails for any service listed in
// failOn, and otherwise records every dispatched batch.
type fakeBatchDispatcher struct {
	failOn      map[string]error
	failBatch   map[string]error
	dispatched  []contracts.Batch
	dispatchSvc []string
}

func (f *fakeBatchDispatcher) Dispatch(ctx context.Context, service string, batch contracts.Batch) (contracts.DispatchAck, error) {
	f.dispatchSvc = append(f.dispatchSvc, service)
	if err, ok := f.failOn[service]; ok {
		return contracts.DispatchAck{}, err
	}
	for suffix, err := range f.failBatch {
		if strings.HasSuffix(batch.BatchID, suffix) {
			return contracts.DispatchAck{}, err
		}
	}
	f.dispatched = append(f.dispatched, batch)
	rows := 0
	for _, dataset := range batch.Datasets {
		rows += dataset.RowCount
	}
	return contracts.DispatchAck{Service: service, DatasetCount: len(batch.Datasets), TotalRowCount: rows}, nil
}

func validRequest(resources ...string) domain.OpenF1IngestionRequest {
	return domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998, Resources: resources}
}

func resultFor(t *testing.T, result domain.OpenF1IngestionResult, resource string) domain.ResourceIngestion {
	t.Helper()
	for _, r := range result.Resources {
		if r.Resource == resource {
			return r
		}
	}
	t.Fatalf("no result for %s in %+v", resource, result.Resources)
	return domain.ResourceIngestion{}
}

// TestIngestOpenF1UseCase_FetchFailureIsIsolated proves one resource failing to fetch is
// reported as failed while the other resources are still fetched, dispatched and reported ok,
// and the run is partial.
func TestIngestOpenF1UseCase_FetchFailureIsIsolated(t *testing.T) {
	provider := &fakeOpenF1Provider{failOn: map[string]error{"laps": errors.New("openf1 unreachable")}}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, dispatcher)

	result, err := uc.Execute(context.Background(), validRequest("drivers", "laps", "meetings"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != domain.IngestionStatusPartial {
		t.Fatalf("status = %q, want partial", result.Status)
	}
	if laps := resultFor(t, result, "laps"); laps.Status != domain.ResourceStatusFailed || laps.Error != "OpenF1 request failed" || laps.Dispatched {
		t.Fatalf("laps = %+v, want failed, not dispatched", laps)
	}
	if drivers := resultFor(t, result, "drivers"); drivers.Status != domain.ResourceStatusOK || !drivers.Dispatched {
		t.Fatalf("drivers = %+v, want ok and dispatched", drivers)
	}
	if strings.Contains(fmt.Sprint(result.Resources), "unreachable") {
		t.Fatal("technical error text leaked into the result")
	}
}

// TestIngestOpenF1UseCase_MappingFailureIsIsolated proves a mapping failure only fails its own
// resource.
func TestIngestOpenF1UseCase_MappingFailureIsIsolated(t *testing.T) {
	mapper := &fakeProviderMapper{failOn: map[string]error{"laps": errors.New("bad row")}}
	uc := NewIngestOpenF1UseCase(&fakeOpenF1Provider{}, mapper, &fakeBatchDispatcher{})

	result, err := uc.Execute(context.Background(), validRequest("drivers", "laps"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if laps := resultFor(t, result, "laps"); laps.Status != domain.ResourceStatusFailed || laps.Error != "mapping failed" {
		t.Fatalf("laps = %+v", laps)
	}
	if resultFor(t, result, "drivers").Status != domain.ResourceStatusOK {
		t.Fatal("drivers should be ok")
	}
}

// TestIngestOpenF1UseCase_DispatchFailureMarksItsResources proves a failed dispatch to one
// service fails only the resources targeting it; the other service is still dispatched.
func TestIngestOpenF1UseCase_DispatchFailureMarksItsResources(t *testing.T) {
	dispatcher := &fakeBatchDispatcher{failOn: map[string]error{"race-data-service": errors.New("down")}}
	uc := NewIngestOpenF1UseCase(&fakeOpenF1Provider{}, &fakeProviderMapper{}, dispatcher)

	result, err := uc.Execute(context.Background(), validRequest("drivers", "laps"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if laps := resultFor(t, result, "laps"); laps.Status != domain.ResourceStatusFailed || laps.Error != "dispatch to race-data-service failed" || laps.Dispatched {
		t.Fatalf("laps = %+v", laps)
	}
	if drivers := resultFor(t, result, "drivers"); !drivers.Dispatched || drivers.Status != domain.ResourceStatusOK {
		t.Fatalf("drivers = %+v", drivers)
	}
}

// TestIngestOpenF1UseCase_EveryResourceFailedIsAnError proves a run where nothing could be
// ingested is an error (the controller answers 502), not a result made only of failures.
func TestIngestOpenF1UseCase_EveryResourceFailedIsAnError(t *testing.T) {
	provider := &fakeOpenF1Provider{failOn: map[string]error{"laps": errors.New("x"), "drivers": errors.New("y")}}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, &fakeBatchDispatcher{})

	if _, err := uc.Execute(context.Background(), validRequest("drivers", "laps")); !errors.Is(err, domain.ErrIngestionFailed) {
		t.Fatalf("err = %v, want ErrIngestionFailed", err)
	}
}

// TestIngestOpenF1UseCase_UnsupportedResourceRefusedUpfront proves an unknown resource name is
// refused with ErrUnsupportedResource before any OpenF1 call.
func TestIngestOpenF1UseCase_UnsupportedResourceRefusedUpfront(t *testing.T) {
	provider := &fakeOpenF1Provider{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, &fakeBatchDispatcher{})

	if _, err := uc.Execute(context.Background(), validRequest("laps", "tyres")); !errors.Is(err, domain.ErrUnsupportedResource) {
		t.Fatalf("err = %v, want ErrUnsupportedResource", err)
	}
	if len(provider.calls) != 0 {
		t.Fatalf("OpenF1 called before validation: %v", provider.calls)
	}
}

// TestIngestOpenF1UseCase_FullSessionHeavyResourcesPerDriver proves the full-session path: light
// resources go first, championship-service before race-data-service, then car_data and location
// are fetched and dispatched one driver at a time, each batch carrying only that driver, with
// row counts and per-service totals summed.
func TestIngestOpenF1UseCase_FullSessionHeavyResourcesPerDriver(t *testing.T) {
	provider := &fakeOpenF1Provider{drivers: []int{1, 44}}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, dispatcher)

	result, err := uc.Execute(context.Background(), validRequest("car_data", "drivers", "laps", "location"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wantCalls := []string{"drivers", "laps", "driver-list", "car_data/1", "car_data/44", "location/1", "location/44"}
	if fmt.Sprint(provider.calls) != fmt.Sprint(wantCalls) {
		t.Fatalf("calls = %v, want %v", provider.calls, wantCalls)
	}
	wantServices := []string{"championship-service", "race-data-service", "race-data-service", "race-data-service", "race-data-service", "race-data-service"}
	if fmt.Sprint(dispatcher.dispatchSvc) != fmt.Sprint(wantServices) {
		t.Fatalf("dispatches = %v, want %v", dispatcher.dispatchSvc, wantServices)
	}
	for _, batch := range dispatcher.dispatched[2:] {
		if batch.Context.DriverNumber == nil || len(batch.Datasets) != 1 {
			t.Fatalf("heavy batch %s is not scoped to one driver: %+v", batch.BatchID, batch.Context)
		}
	}
	carData := resultFor(t, result, "car_data")
	if carData.Status != domain.ResourceStatusOK || carData.RowCount != 2 || !carData.Dispatched || carData.TargetService != "race-data-service" {
		t.Fatalf("car_data = %+v", carData)
	}
	if result.Status != domain.IngestionStatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if len(result.Dispatches) != 2 || result.Dispatches[1].Service != "race-data-service" || result.Dispatches[1].DatasetCount != 5 {
		t.Fatalf("dispatch totals = %+v", result.Dispatches)
	}
}

// TestIngestOpenF1UseCase_HeavyDriverFailureIsPartial proves one driver failing (fetch or
// dispatch) makes the heavy resource partial with that driver listed, without stopping the
// others; all drivers failing makes it failed.
func TestIngestOpenF1UseCase_HeavyDriverFailureIsPartial(t *testing.T) {
	provider := &fakeOpenF1Provider{drivers: []int{1, 44, 63}, failOn: map[string]error{
		"car_data/44": errors.New("429"),
		"location/1":  errors.New("x"), "location/44": errors.New("x"), "location/63": errors.New("x"),
	}}
	dispatcher := &fakeBatchDispatcher{failBatch: map[string]error{"-car_data-63": errors.New("down")}}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, dispatcher)

	result, err := uc.Execute(context.Background(), validRequest("laps", "car_data", "location"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	carData := resultFor(t, result, "car_data")
	if carData.Status != domain.ResourceStatusPartial || fmt.Sprint(carData.FailedDrivers) != "[44 63]" || carData.RowCount != 1 {
		t.Fatalf("car_data = %+v, want partial with drivers 44 and 63 failed", carData)
	}
	if location := resultFor(t, result, "location"); location.Status != domain.ResourceStatusFailed {
		t.Fatalf("location = %+v, want failed", location)
	}
	if result.Status != domain.IngestionStatusPartial {
		t.Fatalf("status = %q", result.Status)
	}
}

// TestIngestOpenF1UseCase_DriverListFailure proves a failing driver list fails the heavy
// resources only.
func TestIngestOpenF1UseCase_DriverListFailure(t *testing.T) {
	provider := &fakeOpenF1Provider{driversErr: fmt.Errorf("drivers: %w", domain.ErrResponseTooLarge)}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, &fakeBatchDispatcher{})

	result, err := uc.Execute(context.Background(), validRequest("laps", "car_data"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if carData := resultFor(t, result, "car_data"); carData.Status != domain.ResourceStatusFailed || carData.Error != "driver list unavailable: OpenF1 response too large" {
		t.Fatalf("car_data = %+v", carData)
	}
	if resultFor(t, result, "laps").Status != domain.ResourceStatusOK {
		t.Fatal("laps should be ok")
	}
}

// TestIngestOpenF1UseCase_DriverRequestFetchesHeavyOnce proves a request for one driver fetches
// car_data in a single call (no driver list), as before.
func TestIngestOpenF1UseCase_DriverRequestFetchesHeavyOnce(t *testing.T) {
	provider := &fakeOpenF1Provider{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, &fakeBatchDispatcher{})
	request := validRequest("car_data")
	driver := 44
	request.DriverNumber = &driver

	if _, err := uc.Execute(context.Background(), request); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if fmt.Sprint(provider.calls) != "[car_data/44]" {
		t.Fatalf("calls = %v, want one car_data call for driver 44", provider.calls)
	}
}

// TestIngestOpenF1UseCase_CancelledContextIsAnError proves a cancelled run (client gone, deadline)
// is returned as the context error instead of a partial result.
func TestIngestOpenF1UseCase_CancelledContextIsAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	uc := NewIngestOpenF1UseCase(&fakeOpenF1Provider{}, &fakeProviderMapper{}, &fakeBatchDispatcher{})

	if _, err := uc.Execute(ctx, validRequest("laps")); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// TestIngestOpenF1UseCase_DispatchDisabledSkipsDispatcher proves dispatch:false on the request
// suppresses every Dispatch call while still fetching/mapping every resource.
func TestIngestOpenF1UseCase_DispatchDisabledSkipsDispatcher(t *testing.T) {
	provider := &fakeOpenF1Provider{drivers: []int{1}}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, dispatcher)

	dispatchFalse := false
	request := validRequest("drivers", "car_data")
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
	for _, r := range result.Resources {
		if r.Dispatched || r.Status != domain.ResourceStatusOK {
			t.Fatalf("resource %+v, want ok and not dispatched", r)
		}
	}
}

// emptyProvider returns no rows for the resources in empty, one row otherwise.
type emptyProvider struct {
	fakeOpenF1Provider
	empty map[string]bool
}

func (e *emptyProvider) Fetch(ctx context.Context, resource string, request domain.OpenF1IngestionRequest) ([]map[string]any, error) {
	rows, err := e.fakeOpenF1Provider.Fetch(ctx, resource, request)
	key := resource
	if request.DriverNumber != nil {
		key = resource + "/" + strconv.Itoa(*request.DriverNumber)
	}
	if e.empty[key] || e.empty[resource] {
		return []map[string]any{}, err
	}
	return rows, err
}

// TestIngestOpenF1UseCase_EmptyResourceIsOkAndNotDispatched proves an empty resource (OpenF1 had
// nothing) is reported ok with no rows and is never dispatched, so it cannot wipe rows stored by
// an earlier ingestion; an empty driver of a heavy resource is not a failed driver either.
func TestIngestOpenF1UseCase_EmptyResourceIsOkAndNotDispatched(t *testing.T) {
	provider := &emptyProvider{fakeOpenF1Provider: fakeOpenF1Provider{drivers: []int{1, 44}}, empty: map[string]bool{"laps": true, "car_data/44": true}}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, dispatcher)

	result, err := uc.Execute(context.Background(), validRequest("laps", "car_data"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if laps := resultFor(t, result, "laps"); laps.Status != domain.ResourceStatusOK || laps.RowCount != 0 || laps.Dispatched {
		t.Fatalf("laps = %+v, want ok, 0 rows, not dispatched", laps)
	}
	carData := resultFor(t, result, "car_data")
	if carData.Status != domain.ResourceStatusOK || len(carData.FailedDrivers) != 0 || carData.RowCount != 1 {
		t.Fatalf("car_data = %+v, want ok with driver 1's row only", carData)
	}
	for _, batch := range dispatcher.dispatched {
		for _, dataset := range batch.Datasets {
			if dataset.RowCount == 0 {
				t.Fatalf("empty dataset %s dispatched in %s", dataset.Name, batch.BatchID)
			}
		}
	}
	if result.Status != domain.IngestionStatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}
}

// TestIngestOpenF1UseCase_UnknownSessionIsAnError proves a session OpenF1 does not know (every
// resource empty) is ErrNoOpenF1Data, not a "completed" run with zero rows.
func TestIngestOpenF1UseCase_UnknownSessionIsAnError(t *testing.T) {
	provider := &emptyProvider{empty: map[string]bool{"laps": true, "drivers": true, "car_data": true}}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, dispatcher)

	if _, err := uc.Execute(context.Background(), validRequest("drivers", "laps", "car_data")); !errors.Is(err, domain.ErrNoOpenF1Data) {
		t.Fatalf("err = %v, want ErrNoOpenF1Data", err)
	}
	if len(dispatcher.dispatchSvc) != 0 {
		t.Fatalf("dispatched %v for an empty session", dispatcher.dispatchSvc)
	}
}

// TestIngestOpenF1UseCase_UnknownSessionStopsBeforeDispatch proves an empty `sessions` resource
// (OpenF1 does not know the session, even if the meeting exists) is ErrNoOpenF1Data and nothing
// is dispatched, not even the meeting.
func TestIngestOpenF1UseCase_UnknownSessionStopsBeforeDispatch(t *testing.T) {
	provider := &emptyProvider{fakeOpenF1Provider: fakeOpenF1Provider{drivers: []int{1}}, empty: map[string]bool{"sessions": true}}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, dispatcher)

	if _, err := uc.Execute(context.Background(), validRequest("meetings", "sessions", "laps", "car_data")); !errors.Is(err, domain.ErrNoOpenF1Data) {
		t.Fatalf("err = %v, want ErrNoOpenF1Data", err)
	}
	if len(dispatcher.dispatchSvc) != 0 {
		t.Fatalf("dispatched %v for an unknown session", dispatcher.dispatchSvc)
	}
}

// TestIngestOpenF1UseCase_EmptyResourceNotMarkedDispatched proves an empty resource targeting a
// service that does receive a batch is not reported as dispatched.
func TestIngestOpenF1UseCase_EmptyResourceNotMarkedDispatched(t *testing.T) {
	provider := &emptyProvider{empty: map[string]bool{"drivers": true}}
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(provider, &fakeProviderMapper{}, dispatcher)

	result, err := uc.Execute(context.Background(), validRequest("meetings", "drivers"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !resultFor(t, result, "meetings").Dispatched {
		t.Fatal("meetings should be dispatched")
	}
	if drivers := resultFor(t, result, "drivers"); drivers.Dispatched || drivers.RowCount != 0 {
		t.Fatalf("drivers = %+v, want not dispatched", drivers)
	}
}

// droppingMapper maps every resource to a dataset without rows (all rows rejected).
type droppingMapper struct{ fakeProviderMapper }

func (d *droppingMapper) Map(resource string, rows []map[string]any) (contracts.Dataset, error) {
	dataset, err := d.fakeProviderMapper.Map(resource, rows)
	dataset.Rows, dataset.RowCount = nil, 0
	return dataset, err
}

// TestIngestOpenF1UseCase_MappedEmptyIsNotDispatched proves the empty check uses the mapped rows:
// rows the mapper drops never produce an empty dispatch that would wipe stored data.
func TestIngestOpenF1UseCase_MappedEmptyIsNotDispatched(t *testing.T) {
	dispatcher := &fakeBatchDispatcher{}
	uc := NewIngestOpenF1UseCase(&fakeOpenF1Provider{drivers: []int{1}}, &droppingMapper{}, dispatcher)

	_, _ = uc.Execute(context.Background(), validRequest("laps", "car_data"))
	if len(dispatcher.dispatchSvc) != 0 {
		t.Fatalf("dispatched %v with no mapped row", dispatcher.dispatchSvc)
	}
}
