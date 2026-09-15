/**
##
## OverDrive 2026
## All Technical rights reserved
##
## accept_ingestion_batch_test.go - Package usecases source file for services/race-data-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

// fakeRaceDataIngestionRepository implements ports.RaceDataIngestionRepository.
type fakeRaceDataIngestionRepository struct {
	err         error
	storedBatch contracts.Batch
	called      bool
}

func (f *fakeRaceDataIngestionRepository) StoreBatch(ctx context.Context, batch contracts.Batch) error {
	f.called = true
	f.storedBatch = batch
	return f.err
}

// fakeAcceptBatchBroadcaster implements ports.RaceControlBroadcaster, recording Publish calls.
type fakeAcceptBatchBroadcaster struct {
	publishedSessionID string
	publishedEvents    []domain.RaceControlEvent
	publishCount       int
}

func (f *fakeAcceptBatchBroadcaster) Publish(sessionID string, events []domain.RaceControlEvent) {
	f.publishCount++
	f.publishedSessionID = sessionID
	f.publishedEvents = events
}

func (f *fakeAcceptBatchBroadcaster) Wait(ctx context.Context, sessionID string, timeout time.Duration) []domain.RaceControlEvent {
	return nil
}

func validBatch(datasetName string, rows []map[string]any) contracts.Batch {
	return contracts.Batch{
		BatchID:  "batch-1",
		Provider: "openf1",
		Context:  contracts.Context{SessionKey: 9998},
		Datasets: []contracts.Dataset{
			{Name: datasetName, TargetService: "race-data-service", RowCount: len(rows), Rows: rows},
		},
	}
}

// TestAcceptIngestionBatchUseCase_Execute_ValidatesInput proves batch_id, at-least-one-dataset,
// and target-service validation all reject before ever touching the repository.
func TestAcceptIngestionBatchUseCase_Execute_ValidatesInput(t *testing.T) {
	t.Run("missing batch_id", func(t *testing.T) {
		repo := &fakeRaceDataIngestionRepository{}
		uc := NewAcceptIngestionBatchUseCase("race-data-service", repo, nil)
		batch := validBatch("laps", nil)
		batch.BatchID = ""
		if _, err := uc.Execute(context.Background(), batch); err == nil {
			t.Fatal("expected an error for a missing batch_id")
		}
		if repo.called {
			t.Fatal("expected the repository not to be called for an invalid batch")
		}
	})

	t.Run("no datasets", func(t *testing.T) {
		repo := &fakeRaceDataIngestionRepository{}
		uc := NewAcceptIngestionBatchUseCase("race-data-service", repo, nil)
		batch := validBatch("laps", nil)
		batch.Datasets = nil
		if _, err := uc.Execute(context.Background(), batch); err == nil {
			t.Fatal("expected an error for zero datasets")
		}
		if repo.called {
			t.Fatal("expected the repository not to be called for an invalid batch")
		}
	})

	t.Run("wrong target service", func(t *testing.T) {
		repo := &fakeRaceDataIngestionRepository{}
		uc := NewAcceptIngestionBatchUseCase("race-data-service", repo, nil)
		batch := validBatch("laps", nil)
		batch.Datasets[0].TargetService = "championship-service"
		if _, err := uc.Execute(context.Background(), batch); err == nil {
			t.Fatal("expected an error for a dataset targeting a different service")
		}
		if repo.called {
			t.Fatal("expected the repository not to be called for an invalid batch")
		}
	})
}

// TestAcceptIngestionBatchUseCase_Execute_RepositoryFailurePropagates proves a StoreBatch failure
// is surfaced, not swallowed, and that no race control event is published in that case.
func TestAcceptIngestionBatchUseCase_Execute_RepositoryFailurePropagates(t *testing.T) {
	boom := errors.New("db unavailable")
	repo := &fakeRaceDataIngestionRepository{err: boom}
	broadcaster := &fakeAcceptBatchBroadcaster{}
	uc := NewAcceptIngestionBatchUseCase("race-data-service", repo, broadcaster)

	_, err := uc.Execute(context.Background(), validBatch("laps", nil))
	if !errors.Is(err, boom) {
		t.Fatalf("expected the repository error to propagate, got %v", err)
	}
	if broadcaster.publishCount != 0 {
		t.Fatalf("expected no publish on a storage failure, got %d calls", broadcaster.publishCount)
	}
}

// TestAcceptIngestionBatchUseCase_Execute_NormalCase proves a valid batch is stored and produces
// a well-formed acknowledgement with dataset/row counts summed from the batch.
func TestAcceptIngestionBatchUseCase_Execute_NormalCase(t *testing.T) {
	repo := &fakeRaceDataIngestionRepository{}
	uc := NewAcceptIngestionBatchUseCase("race-data-service", repo, nil)

	ack, err := uc.Execute(context.Background(), validBatch("laps", []map[string]any{{"lap_number": 1}, {"lap_number": 2}}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ack.Accepted || ack.BatchID != "batch-1" || ack.DatasetCount != 1 || ack.TotalRowCount != 2 {
		t.Fatalf("expected a well-formed ack, got %+v", ack)
	}
	if !repo.called {
		t.Fatal("expected StoreBatch to be called")
	}
}

// TestAcceptIngestionBatchUseCase_Execute_PublishesRaceControlEvents proves a race_control dataset
// batch is mapped into domain.RaceControlEvent rows and published to the broadcaster under the
// deterministic session ID derived from provider+sessionKey, connecting to
// race_control_broadcaster_test.go's Wait-side coverage.
func TestAcceptIngestionBatchUseCase_Execute_PublishesRaceControlEvents(t *testing.T) {
	repo := &fakeRaceDataIngestionRepository{}
	broadcaster := &fakeAcceptBatchBroadcaster{}
	uc := NewAcceptIngestionBatchUseCase("race-data-service", repo, broadcaster)

	batch := validBatch("race_control", []map[string]any{
		{"category": "Flag", "flag": "YELLOW", "message": "yellow flag sector 1", "lap_number": 12, "date_utc": "2026-03-08T15:00:00Z", "scope": "Track"},
	})

	_, err := uc.Execute(context.Background(), batch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if broadcaster.publishCount != 1 {
		t.Fatalf("expected exactly one Publish call, got %d", broadcaster.publishCount)
	}
	wantSessionID := contracts.SessionID("openf1", 9998)
	if broadcaster.publishedSessionID != wantSessionID {
		t.Fatalf("expected sessionID %q, got %q", wantSessionID, broadcaster.publishedSessionID)
	}
	if len(broadcaster.publishedEvents) != 1 || broadcaster.publishedEvents[0].Message != "yellow flag sector 1" {
		t.Fatalf("expected the mapped race control event to be published, got %+v", broadcaster.publishedEvents)
	}
}

// TestAcceptIngestionBatchUseCase_Execute_NonRaceControlDatasetDoesNotPublish proves any other
// dataset name never triggers a broadcaster Publish call.
func TestAcceptIngestionBatchUseCase_Execute_NonRaceControlDatasetDoesNotPublish(t *testing.T) {
	repo := &fakeRaceDataIngestionRepository{}
	broadcaster := &fakeAcceptBatchBroadcaster{}
	uc := NewAcceptIngestionBatchUseCase("race-data-service", repo, broadcaster)

	_, err := uc.Execute(context.Background(), validBatch("laps", []map[string]any{{"lap_number": 1}}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if broadcaster.publishCount != 0 {
		t.Fatalf("expected no publish for a non-race_control dataset, got %d calls", broadcaster.publishCount)
	}
}

// TestAcceptIngestionBatchUseCase_Execute_NilBroadcasterIsSkipped proves a nil broadcaster (as
// documented on NewAcceptIngestionBatchUseCase) is tolerated instead of panicking, even for a
// race_control batch.
func TestAcceptIngestionBatchUseCase_Execute_NilBroadcasterIsSkipped(t *testing.T) {
	repo := &fakeRaceDataIngestionRepository{}
	uc := NewAcceptIngestionBatchUseCase("race-data-service", repo, nil)

	_, err := uc.Execute(context.Background(), validBatch("race_control", []map[string]any{{"message": "flag"}}))
	if err != nil {
		t.Fatalf("expected a nil broadcaster to be tolerated, got %v", err)
	}
}

// TestMapRaceControlEvent proves the raw-row-to-domain mapping, including the safety-car heuristic
// and the date_utc parse failure/absence case (event.Timestamp stays the zero value).
func TestMapRaceControlEvent(t *testing.T) {
	t.Run("full row with virtual safety car scope", func(t *testing.T) {
		row := map[string]any{
			"category": "SafetyCar", "flag": "", "message": "VSC deployed", "lap_number": 5,
			"date_utc": "2026-03-08T15:00:00Z", "scope": "Virtual",
		}
		event := mapRaceControlEvent(row)
		if event.Message != "VSC deployed" || event.LapNumber != 5 {
			t.Fatalf("unexpected mapped event: %+v", event)
		}
		if event.SafetyCar == nil || *event.SafetyCar != "VSC" {
			t.Fatalf("expected safetyCar=VSC, got %v", event.SafetyCar)
		}
		if event.Timestamp.IsZero() {
			t.Fatal("expected a parsed timestamp")
		}
	})

	t.Run("missing date_utc leaves a zero timestamp", func(t *testing.T) {
		event := mapRaceControlEvent(map[string]any{"message": "flag"})
		if !event.Timestamp.IsZero() {
			t.Fatalf("expected a zero timestamp when date_utc is absent, got %v", event.Timestamp)
		}
	})
}

// TestSafetyCarFromScope covers every branch of the heuristic: virtual (VSC), safety car in either
// spelling (SC), and neither (nil).
func TestSafetyCarFromScope(t *testing.T) {
	cases := []struct {
		name     string
		scope    string
		category string
		want     *string
	}{
		{"virtual scope", "Virtual", "", strPtr("VSC")},
		{"safety car with space", "Safety Car", "", strPtr("SC")},
		{"safetycar no space", "", "SafetyCar deployed", strPtr("SC")},
		{"neither", "Track", "Flag", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := safetyCarFromScope(tc.scope, tc.category)
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("safetyCarFromScope(%q, %q) = %v, want %v", tc.scope, tc.category, got, tc.want)
			}
			if got != nil && *got != *tc.want {
				t.Fatalf("safetyCarFromScope(%q, %q) = %q, want %q", tc.scope, tc.category, *got, *tc.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// TestStringifyAny covers the nil/string/other-type branches.
func TestStringifyAny(t *testing.T) {
	if got := stringifyAny(nil); got != "" {
		t.Fatalf("stringifyAny(nil) = %q, want empty string", got)
	}
	if got := stringifyAny("hello"); got != "hello" {
		t.Fatalf("stringifyAny(\"hello\") = %q, want \"hello\"", got)
	}
	if got := stringifyAny(42); got != "42" {
		t.Fatalf("stringifyAny(42) = %q, want \"42\"", got)
	}
}

// TestIntValueFromRow covers every supported dynamic-value branch, plus the unparsable-string and
// unsupported-type fallback to zero.
func TestIntValueFromRow(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{"int", 3, 3},
		{"int32", int32(4), 4},
		{"int64", int64(5), 5},
		{"float64", float64(6.7), 6},
		{"numeric string", "7", 7},
		{"unparsable string", "abc", 0},
		{"unsupported type", true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := intValueFromRow(tc.value); got != tc.want {
				t.Fatalf("intValueFromRow(%#v) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

// TestParseTimeAny proves a valid RFC3339 string parses to UTC, and both an empty value and an
// unparsable string report ok=false rather than a zero-but-valid time.
func TestParseTimeAny(t *testing.T) {
	got, ok := parseTimeAny("2026-03-08T15:00:00Z")
	if !ok || got.Year() != 2026 {
		t.Fatalf("expected a parsed time, got %v ok=%v", got, ok)
	}
	if _, ok := parseTimeAny(nil); ok {
		t.Fatal("expected ok=false for a nil value")
	}
	if _, ok := parseTimeAny("not-a-time"); ok {
		t.Fatal("expected ok=false for an unparsable string")
	}
}
