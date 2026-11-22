/**
##
## OverDrive 2026
## All Technical rights reserved
##
## shared_race_control_broadcaster.go - Multi-instance race control broadcaster polling a Postgres table.
##
*/

package usecases

import (
	"context"
	"log"
	"sync"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// Defaults of the shared broadcaster. A waiter is woken at most PollInterval after the batch is
// stored. Window must exceed the time between an insert and the next poll on every instance (a
// slow poll, a short pause); rows are kept for Retention, far beyond Window.
const (
	DefaultRaceControlPollInterval = time.Second
	DefaultRaceControlPollWindow   = 15 * time.Second
	DefaultRaceControlRetention    = 5 * time.Minute
	raceControlStoreTimeout        = 5 * time.Second
	raceControlCleanupEvery        = 60
)

// SharedRaceControlBroadcaster makes the race control long poll work across several
// race-data-service instances without new infrastructure: Publish stores the batch in the
// RaceControlNotification table, and every instance polls the rows of the last Window to wake
// its local waiters. Rows are identified by ID, so each batch is delivered once per instance
// even though consecutive polls overlap.
type SharedRaceControlBroadcaster struct {
	store     ports.RaceControlNotificationStore
	local     *RaceControlBroadcaster
	interval  time.Duration
	window    time.Duration
	retention time.Duration

	mu     sync.Mutex
	seen   map[int64]time.Time
	primed bool
}

// NewSharedRaceControlBroadcaster wraps local (which holds this instance's waiters and their
// caps) with the shared store, using the default poll interval, window and retention.
func NewSharedRaceControlBroadcaster(store ports.RaceControlNotificationStore, local *RaceControlBroadcaster) *SharedRaceControlBroadcaster {
	return &SharedRaceControlBroadcaster{
		store:     store,
		local:     local,
		interval:  DefaultRaceControlPollInterval,
		window:    DefaultRaceControlPollWindow,
		retention: DefaultRaceControlRetention,
		seen:      make(map[int64]time.Time),
	}
}

// Publish stores the batch for every instance. If the store is unreachable, the batch is still
// delivered to this instance's waiters so a single-instance deployment keeps working.
func (b *SharedRaceControlBroadcaster) Publish(sessionID string, events []domain.RaceControlEvent) {
	if len(events) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), raceControlStoreTimeout)
	defer cancel()
	if err := b.store.Insert(ctx, sessionID, events); err != nil {
		log.Printf("race control: store batch for session %s failed, waking local waiters only: %v", sessionID, err)
		b.local.Publish(sessionID, events)
	}
}

// Wait blocks one client on this instance (see RaceControlBroadcaster.Wait).
func (b *SharedRaceControlBroadcaster) Wait(ctx context.Context, sessionID string, timeout time.Duration) ([]domain.RaceControlEvent, error) {
	return b.local.Wait(ctx, sessionID, timeout)
}

// Close releases this instance's waiters and refuses new ones (server shutdown hook).
func (b *SharedRaceControlBroadcaster) Close() {
	b.local.Close()
}

// Run polls the store until ctx is done and periodically deletes expired rows. Poll errors are
// logged and retried at the next tick.
func (b *SharedRaceControlBroadcaster) Run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for tick := 0; ; tick++ {
		if err := b.poll(ctx); err != nil && ctx.Err() == nil {
			log.Printf("race control: poll failed: %v", err)
		}
		if tick%raceControlCleanupEvery == 0 {
			b.cleanup(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// poll delivers the batches of the last window not delivered yet. The first poll only records
// what already exists, so a restarting instance does not replay old events to new waiters.
func (b *SharedRaceControlBroadcaster) poll(ctx context.Context) error {
	pollCtx, cancel := context.WithTimeout(ctx, raceControlStoreTimeout)
	defer cancel()
	rows, err := b.store.ListRecent(pollCtx, b.window)
	if err != nil {
		return err
	}

	now := time.Now()
	b.mu.Lock()
	fresh := make([]ports.RaceControlNotification, 0, len(rows))
	for _, row := range rows {
		if _, done := b.seen[row.ID]; done {
			continue
		}
		b.seen[row.ID] = now
		if b.primed {
			fresh = append(fresh, row)
		}
	}
	b.primed = true
	for id, at := range b.seen {
		if now.Sub(at) > 2*b.window {
			delete(b.seen, id)
		}
	}
	b.mu.Unlock()

	for _, row := range fresh {
		b.local.Publish(row.SessionID, row.Events)
	}
	return nil
}

// cleanup deletes the rows older than the retention; any instance may do it, it is idempotent.
func (b *SharedRaceControlBroadcaster) cleanup(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, raceControlStoreTimeout)
	defer cancel()
	if err := b.store.DeleteOlderThan(cleanupCtx, b.retention); err != nil && ctx.Err() == nil {
		log.Printf("race control: cleanup failed: %v", err)
	}
}
