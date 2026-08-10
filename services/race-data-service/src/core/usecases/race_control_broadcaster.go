/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_control_broadcaster.go - Package usecases source file for services/race-data-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"
	"sync"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// RaceControlBroadcaster is an in-memory, per-session pub/sub used to implement
// the long-polling contract of POST /sessions/{sessionId}/race/control:
// the handler blocks until a new RaceControlEvent batch is ingested for that
// session, then returns and closes — the client must re-POST to wait for the
// next event.
//
// IMPORTANT: this is a single-process, in-memory implementation (a Go channel
// fan-out keyed by sessionID.
type RaceControlBroadcaster struct {
	mu          sync.Mutex
	subscribers map[string][]chan []domain.RaceControlEvent
}

// NewRaceControlBroadcaster builds and returns an in-memory race control broadcaster.
func NewRaceControlBroadcaster() *RaceControlBroadcaster {
	return &RaceControlBroadcaster{
		subscribers: make(map[string][]chan []domain.RaceControlEvent),
	}
}

// Publish notifies every current waiter for sessionID and clears them.
// It is safe to call with an empty events slice (no-op).
func (b *RaceControlBroadcaster) Publish(sessionID string, events []domain.RaceControlEvent) {
	if len(events) == 0 {
		return
	}
	b.mu.Lock()
	waiters := b.subscribers[sessionID]
	delete(b.subscribers, sessionID)
	b.mu.Unlock()

	for _, ch := range waiters {
		ch <- events
		close(ch)
	}
}

// Wait blocks until either a new race control event batch is published for
// sessionID, the context is cancelled, or timeout elapses (returning an empty
// slice in the timeout/cancellation case so the caller can respond 200 with
// `[]` and let the client re-POST, per the long-polling contract).
func (b *RaceControlBroadcaster) Wait(ctx context.Context, sessionID string, timeout time.Duration) []domain.RaceControlEvent {
	ch := make(chan []domain.RaceControlEvent, 1)

	b.mu.Lock()
	b.subscribers[sessionID] = append(b.subscribers[sessionID], ch)
	b.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case events := <-ch:
		return events
	case <-ctx.Done():
		b.removeSubscriber(sessionID, ch)
		return nil
	case <-timer.C:
		b.removeSubscriber(sessionID, ch)
		return nil
	}
}

// removeSubscriber implements the remove subscriber workflow for this package.
func (b *RaceControlBroadcaster) removeSubscriber(sessionID string, target chan []domain.RaceControlEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	waiters := b.subscribers[sessionID]
	for i, ch := range waiters {
		if ch == target {
			b.subscribers[sessionID] = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(b.subscribers[sessionID]) == 0 {
		delete(b.subscribers, sessionID)
	}
}
