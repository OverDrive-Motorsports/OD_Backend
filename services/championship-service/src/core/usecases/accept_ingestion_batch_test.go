/**
##
## OverDrive 2026
## All Technical rights reserved
##
## accept_ingestion_batch_test.go - Package usecases source file for services/championship-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	contracts "overdrive/shared/contracts/ingestion"
)

// fakeChampionshipIngestionRepository implements ports.ChampionshipIngestionRepository for
// usecase-level tests, recording the last batch it was asked to store.
type fakeChampionshipIngestionRepository struct {
	err       error
	lastBatch contracts.Batch
	called    bool
}

func (f *fakeChampionshipIngestionRepository) StoreBatch(ctx context.Context, batch contracts.Batch) error {
	f.called = true
	f.lastBatch = batch
	return f.err
}

func newTestBatch(serviceName string) contracts.Batch {
	return contracts.Batch{
		BatchID: "batch-1",
		Datasets: []contracts.Dataset{
			{Name: "event_catalog", TargetService: serviceName, RowCount: 2},
			{Name: "session_catalog", TargetService: serviceName, RowCount: 3},
		},
	}
}

// TestAcceptIngestionBatchUseCase_Execute_Success proves a valid batch is stored via the
// repository and acknowledged with the aggregated dataset/row counts and a UTC processed
// timestamp from the injected clock.
func TestAcceptIngestionBatchUseCase_Execute_Success(t *testing.T) {
	repo := &fakeChampionshipIngestionRepository{}
	uc := NewAcceptIngestionBatchUseCase("championship-service", repo)
	fixedNow := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return fixedNow }

	ack, err := uc.Execute(context.Background(), newTestBatch("championship-service"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.called {
		t.Fatal("expected the repository to be called")
	}
	if ack.Service != "championship-service" || ack.BatchID != "batch-1" || !ack.Accepted {
		t.Fatalf("unexpected ack identity fields: %+v", ack)
	}
	if ack.DatasetCount != 2 || ack.TotalRowCount != 5 {
		t.Fatalf("unexpected aggregated counts: %+v", ack)
	}
	if !ack.ProcessedAtUtc.Equal(fixedNow) {
		t.Fatalf("ProcessedAtUtc = %v, want %v", ack.ProcessedAtUtc, fixedNow)
	}
}

// TestAcceptIngestionBatchUseCase_Execute_Validation proves the up-front validation rejects a
// missing batch id, an empty dataset list, and a dataset targeting a different service - and in
// each case the repository is never called.
func TestAcceptIngestionBatchUseCase_Execute_Validation(t *testing.T) {
	tests := []struct {
		name  string
		batch contracts.Batch
	}{
		{"missing batch id", contracts.Batch{Datasets: []contracts.Dataset{{Name: "event_catalog", TargetService: "championship-service"}}}},
		{"no datasets", contracts.Batch{BatchID: "batch-1"}},
		{"dataset targets a different service", contracts.Batch{
			BatchID:  "batch-1",
			Datasets: []contracts.Dataset{{Name: "car_data", TargetService: "race-data-service"}},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeChampionshipIngestionRepository{}
			uc := NewAcceptIngestionBatchUseCase("championship-service", repo)

			_, err := uc.Execute(context.Background(), tc.batch)
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if repo.called {
				t.Fatal("expected the repository not to be called for an invalid batch")
			}
		})
	}
}

// TestAcceptIngestionBatchUseCase_Execute_RepositoryErrorPropagates proves a repository failure
// is surfaced to the caller unwrapped, and no ack is fabricated on failure.
func TestAcceptIngestionBatchUseCase_Execute_RepositoryErrorPropagates(t *testing.T) {
	wantErr := errors.New("store failed")
	repo := &fakeChampionshipIngestionRepository{err: wantErr}
	uc := NewAcceptIngestionBatchUseCase("championship-service", repo)

	ack, err := uc.Execute(context.Background(), newTestBatch("championship-service"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the repository error to propagate, got %v", err)
	}
	if ack.Accepted {
		t.Fatalf("expected a zero-value ack on failure, got %+v", ack)
	}
}
