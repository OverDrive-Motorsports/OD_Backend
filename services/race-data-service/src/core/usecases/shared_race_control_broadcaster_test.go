/**
##
## OverDrive 2026
## All Technical rights reserved
##
## shared_race_control_broadcaster_test.go - Tests for the multi-instance race control broadcaster.
##
*/

package usecases

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// fakeNotificationStore is an in-memory ports.RaceControlNotificationStore shared by several
// broadcasters, standing for the table that every instance reads.
type fakeNotificationStore struct {
	mu        sync.Mutex
	rows      []ports.RaceControlNotification
	nextID    int64
	insertErr error
	listErr   error
	deleted   []time.Duration
}

func (f *fakeNotificationStore) Insert(ctx context.Context, sessionID string, events []domain.RaceControlEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return f.insertErr
	}
	f.nextID++
	f.rows = append(f.rows, ports.RaceControlNotification{ID: f.nextID, SessionID: sessionID, Events: events})
	return nil
}

func (f *fakeNotificationStore) ListRecent(ctx context.Context, window time.Duration) ([]ports.RaceControlNotification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]ports.RaceControlNotification(nil), f.rows...), nil
}

func (f *fakeNotificationStore) DeleteOlderThan(ctx context.Context, age time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, age)
	return nil
}

// waitAsync starts a waiter on b and returns the channel receiving its events.
func waitAsync(b *SharedRaceControlBroadcaster, sessionID string, timeout time.Duration) <-chan []domain.RaceControlEvent {
	done := make(chan []domain.RaceControlEvent, 1)
	go func() { done <- eventsOnly(b.Wait(context.Background(), sessionID, timeout)) }()
	return done
}

// TestSharedRaceControlBroadcaster_WakesWaiterOnOtherInstance proves the multi-instance
// contract: a batch published through instance A wakes a long poll waiting on instance B once B
// polls the shared store.
func TestSharedRaceControlBroadcaster_WakesWaiterOnOtherInstance(t *testing.T) {
	store := &fakeNotificationStore{}
	instanceA := NewSharedRaceControlBroadcaster(store, NewRaceControlBroadcaster())
	instanceB := NewSharedRaceControlBroadcaster(store, NewRaceControlBroadcaster())
	if err := instanceB.poll(context.Background()); err != nil {
		t.Fatalf("priming poll: %v", err)
	}
	done := waitAsync(instanceB, "session-A", 5*time.Second)
	waitUntilSubscriberCount(t, instanceB.local, "session-A", 1)

	instanceA.Publish("session-A", event("red flag"))
	if err := instanceB.poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}

	select {
	case got := <-done:
		if len(got) != 1 || got[0].Message != "red flag" {
			t.Fatalf("waiter on B got %+v, want the batch published on A", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter on instance B was not woken")
	}
}

// TestSharedRaceControlBroadcaster_DeliversEachBatchOnce proves overlapping polls (the window
// covers several ticks) never deliver the same stored batch twice to one instance.
func TestSharedRaceControlBroadcaster_DeliversEachBatchOnce(t *testing.T) {
	store := &fakeNotificationStore{}
	b := NewSharedRaceControlBroadcaster(store, NewRaceControlBroadcaster())
	if err := b.poll(context.Background()); err != nil {
		t.Fatalf("priming poll: %v", err)
	}
	b.Publish("session-A", event("yellow"))
	if err := b.poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}

	done := waitAsync(b, "session-A", 150*time.Millisecond)
	waitUntilSubscriberCount(t, b.local, "session-A", 1)
	if err := b.poll(context.Background()); err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if got := <-done; got != nil {
		t.Fatalf("a later waiter received %+v, the batch was delivered twice", got)
	}
}

// TestSharedRaceControlBroadcaster_FirstPollDoesNotReplay proves a starting instance only records
// the batches already in the store: a client that starts waiting after the restart is not woken
// by an old event.
func TestSharedRaceControlBroadcaster_FirstPollDoesNotReplay(t *testing.T) {
	store := &fakeNotificationStore{}
	_ = store.Insert(context.Background(), "session-A", event("old"))
	b := NewSharedRaceControlBroadcaster(store, NewRaceControlBroadcaster())

	done := waitAsync(b, "session-A", 150*time.Millisecond)
	waitUntilSubscriberCount(t, b.local, "session-A", 1)
	if err := b.poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if got := <-done; got != nil {
		t.Fatalf("waiter received %+v from before the instance started", got)
	}
}

// TestSharedRaceControlBroadcaster_StoreDownFallsBackToLocal proves a failed insert still wakes
// the waiters of the publishing instance, so a single instance keeps working without the table.
func TestSharedRaceControlBroadcaster_StoreDownFallsBackToLocal(t *testing.T) {
	store := &fakeNotificationStore{insertErr: errors.New("db down")}
	b := NewSharedRaceControlBroadcaster(store, NewRaceControlBroadcaster())
	done := waitAsync(b, "session-A", 5*time.Second)
	waitUntilSubscriberCount(t, b.local, "session-A", 1)

	b.Publish("session-A", event("safety car"))

	select {
	case got := <-done:
		if len(got) != 1 {
			t.Fatalf("got %+v, want the batch", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("local waiter not woken when the store is down")
	}
}

// TestSharedRaceControlBroadcaster_PollError proves a failing poll is reported and delivers
// nothing.
func TestSharedRaceControlBroadcaster_PollError(t *testing.T) {
	store := &fakeNotificationStore{listErr: errors.New("db down")}
	b := NewSharedRaceControlBroadcaster(store, NewRaceControlBroadcaster())
	if err := b.poll(context.Background()); err == nil {
		t.Fatal("poll() = nil, want the store error")
	}
}

// TestSharedRaceControlBroadcaster_RunPollsAndCleansUp proves Run polls on its own (a waiter is
// woken without calling poll by hand), deletes rows past the retention, and stops with its
// context.
func TestSharedRaceControlBroadcaster_RunPollsAndCleansUp(t *testing.T) {
	store := &fakeNotificationStore{}
	b := NewSharedRaceControlBroadcaster(store, NewRaceControlBroadcaster())
	b.interval = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		b.Run(ctx)
		close(stopped)
	}()

	done := waitAsync(b, "session-A", 5*time.Second)
	waitUntilSubscriberCount(t, b.local, "session-A", 1)
	time.Sleep(30 * time.Millisecond) // let the first (priming) poll run
	b.Publish("session-A", event("chequered flag"))

	select {
	case got := <-done:
		if len(got) != 1 {
			t.Fatalf("got %+v, want the batch", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not deliver the batch")
	}
	cancel()
	<-stopped

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.deleted) == 0 || store.deleted[0] != DefaultRaceControlRetention {
		t.Fatalf("cleanup calls = %v, want one with the retention", store.deleted)
	}
}

// TestSharedRaceControlBroadcaster_CloseReleasesWaiters proves the shutdown hook reaches the
// local waiters.
func TestSharedRaceControlBroadcaster_CloseReleasesWaiters(t *testing.T) {
	b := NewSharedRaceControlBroadcaster(&fakeNotificationStore{}, NewRaceControlBroadcaster())
	done := waitAsync(b, "session-A", 10*time.Second)
	waitUntilSubscriberCount(t, b.local, "session-A", 1)

	b.Close()

	select {
	case got := <-done:
		if got != nil {
			t.Fatalf("got %+v, want an empty release", got)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not release the waiter")
	}
}
