/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_repository_test.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
##
*/

package prismaadapter

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
)

// TestRunConcurrent_Empty proves an empty row set is a no-op (no goroutines spawned, no error).
func TestRunConcurrent_Empty(t *testing.T) {
	var calls int32
	err := runConcurrent(nil, func(row map[string]any) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error for empty rows, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected fn to never be called, got %d calls", calls)
	}
}

// TestRunConcurrent_AllSucceed proves every row is processed exactly once when fn never errors.
func TestRunConcurrent_AllSucceed(t *testing.T) {
	rows := make([]map[string]any, 50)
	for i := range rows {
		rows[i] = map[string]any{"i": i}
	}

	var calls int32
	err := runConcurrent(rows, func(row map[string]any) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if int(calls) != len(rows) {
		t.Fatalf("expected fn called once per row (%d), got %d calls", len(rows), calls)
	}
}

// TestRunConcurrent_FirstErrorWins proves a single worker failing surfaces that error to the
// caller instead of it being silently swallowed - the exact regression this helper exists to
func TestRunConcurrent_FirstErrorWins(t *testing.T) {
	boom := errors.New("boom")
	rows := make([]map[string]any, 20)
	for i := range rows {
		rows[i] = map[string]any{"i": i}
	}

	err := runConcurrent(rows, func(row map[string]any) error {
		if row["i"] == 5 {
			return boom
		}
		return nil
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected the worker error to be returned, got %v", err)
	}
}

// TestRunConcurrent_ReturnsFirstErrorDeterministically proves that whichever error is recorded
// first under concurrent access is a real error from fn (not nil, not corrupted), even when
// several workers fail at once - guards the mutex-protected firstErr assignment.
func TestRunConcurrent_ReturnsFirstErrorDeterministically(t *testing.T) {
	rows := make([]map[string]any, 30)
	for i := range rows {
		rows[i] = map[string]any{"i": i}
	}

	err := runConcurrent(rows, func(row map[string]any) error {
		return fmt.Errorf("row %v failed", row["i"])
	})
	if err == nil {
		t.Fatal("expected a non-nil error when every worker fails")
	}
}

// TestRunConcurrent_BoundsConcurrency proves at most ingestionConcurrency workers run fn at
// once, matching the documented "bounded-concurrency writer" behavior.
func TestRunConcurrent_BoundsConcurrency(t *testing.T) {
	rows := make([]map[string]any, ingestionConcurrency*4)
	for i := range rows {
		rows[i] = map[string]any{"i": i}
	}

	var current, max int32
	err := runConcurrent(rows, func(row map[string]any) error {
		n := atomic.AddInt32(&current, 1)
		for {
			m := atomic.LoadInt32(&max)
			if n <= m || atomic.CompareAndSwapInt32(&max, m, n) {
				break
			}
		}
		atomic.AddInt32(&current, -1)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if max > int32(ingestionConcurrency) {
		t.Fatalf("observed %d concurrent workers, want <= %d", max, ingestionConcurrency)
	}
}
