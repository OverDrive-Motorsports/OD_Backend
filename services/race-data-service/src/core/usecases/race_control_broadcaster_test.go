/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_control_broadcaster_test.go - Package usecases source file for services/race-data-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"sync"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// event builds a single-element RaceControlEvent slice used to Publish in these tests.
func event(message string) []domain.RaceControlEvent {
	return []domain.RaceControlEvent{{Message: message}}
}

// TestRaceControlBroadcaster_PublishWakesConcurrentWaiters proves that a single Publish call
// wakes every concurrent Wait call currently blocked on that session, each receiving the same
// event batch - the core of the POST /race/control long-poll contract.
func TestRaceControlBroadcaster_PublishWakesConcurrentWaiters(t *testing.T) {
	b := NewRaceControlBroadcaster()
	const waiters = 5

	var wg sync.WaitGroup
	results := make([][]domain.RaceControlEvent, waiters)
	ready := make(chan struct{}, waiters)

	for i := 0; i < waiters; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ready <- struct{}{}
			results[i] = b.Wait(context.Background(), "session-A", 5*time.Second)
		}(i)
	}

	// Wait for every goroutine to have started (not a guarantee they've reached
	// Wait's internal subscribe yet, so also poll subscriber count below).
	for i := 0; i < waiters; i++ {
		<-ready
	}
	waitUntilSubscriberCount(t, b, "session-A", waiters)

	b.Publish("session-A", event("SC deployed"))
	wg.Wait()

	for i, got := range results {
		if len(got) != 1 || got[0].Message != "SC deployed" {
			t.Fatalf("waiter %d: expected to receive the published event, got %+v", i, got)
		}
	}
}

// TestRaceControlBroadcaster_TimeoutReturnsEmptyAfterRealElapsedTime proves Wait actually blocks
// for (approximately) the requested timeout and returns an empty slice, not an early/instant
// return, when nothing is ever published. Asserts real elapsed wall-clock time, in the style of
// the gateway's flush_passthrough_test.go.
func TestRaceControlBroadcaster_TimeoutReturnsEmptyAfterRealElapsedTime(t *testing.T) {
	b := NewRaceControlBroadcaster()
	const timeout = 150 * time.Millisecond

	start := time.Now()
	got := b.Wait(context.Background(), "session-B", timeout)
	elapsed := time.Since(start)

	if got != nil {
		t.Fatalf("expected nil/empty events on timeout, got %+v", got)
	}
	if elapsed < timeout {
		t.Fatalf("Wait returned after %v, want at least the requested timeout %v", elapsed, timeout)
	}
	// Generous upper bound to catch a broken implementation that never returns/blocks far
	// longer than requested, without being flaky under CI scheduling jitter.
	if elapsed > timeout+2*time.Second {
		t.Fatalf("Wait returned after %v, want close to the requested timeout %v", elapsed, timeout)
	}
}

// TestRaceControlBroadcaster_ContextCancellationReturnsPromptlyAndCleansUp proves Wait respects
// context cancellation independently of the timeout (returns well before a long timeout would
// have elapsed) and removes its subscriber entry so a cancelled waiter isn't leaked forever.
func TestRaceControlBroadcaster_ContextCancellationReturnsPromptlyAndCleansUp(t *testing.T) {
	b := NewRaceControlBroadcaster()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan []domain.RaceControlEvent, 1)
	go func() {
		done <- b.Wait(ctx, "session-C", 10*time.Second)
	}()

	waitUntilSubscriberCount(t, b, "session-C", 1)

	start := time.Now()
	cancel()

	select {
	case got := <-done:
		if got != nil {
			t.Fatalf("expected nil events on cancellation, got %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return promptly after context cancellation")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Wait took %v to return after cancellation, want near-instant", elapsed)
	}

	waitUntilSubscriberCount(t, b, "session-C", 0)
}

// TestRaceControlBroadcaster_SessionIsolation proves an event published for session A never
func TestRaceControlBroadcaster_SessionIsolation(t *testing.T) {
	b := NewRaceControlBroadcaster()

	done := make(chan []domain.RaceControlEvent, 1)
	go func() {
		done <- b.Wait(context.Background(), "session-B", 300*time.Millisecond)
	}()

	waitUntilSubscriberCount(t, b, "session-B", 1)

	// Published for a different session - must not wake the session-B waiter.
	b.Publish("session-A", event("should not be seen by B"))

	select {
	case got := <-done:
		if got != nil {
			t.Fatalf("session-B waiter was woken by a session-A publish: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session-B waiter never returned (timeout logic broken)")
	}
}

// waitUntilSubscriberCount polls the broadcaster's internal subscriber map until it reaches the
// expected count for sessionID or fails the test after a bounded number of attempts. Used only
// to make these tests deterministic without exposing new production API surface for testing.
func waitUntilSubscriberCount(t *testing.T, b *RaceControlBroadcaster, sessionID string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		got := len(b.subscribers[sessionID])
		b.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for subscriber count %d on %q", want, sessionID)
}
