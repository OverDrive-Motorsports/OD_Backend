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

// Default caps of the race control long poll on one instance. Each waiter is an open HTTP
// connection and a goroutine, so the caps bound what a burst of clients can hold.
const (
	DefaultRaceControlMaxWaiters           = 2000
	DefaultRaceControlMaxWaitersPerSession = 500
)

// RaceControlBroadcaster is the in-process, per-session fan-out behind the long-polling
// contract of POST /sessions/{sessionId}/race/control: the handler blocks until a new
// RaceControlEvent batch is published for that session, then returns and closes; the client
// must re-POST to wait for the next event.
//
// On its own it only sees events published in the same process. In production it is wrapped by
// SharedRaceControlBroadcaster, which publishes through Postgres so that every race-data-service
// instance wakes its local waiters; tests use it directly.
type RaceControlBroadcaster struct {
	mu            sync.Mutex
	subscribers   map[string][]chan []domain.RaceControlEvent
	waiters       int
	maxWaiters    int
	maxPerSession int
	closed        bool
}

// NewRaceControlBroadcaster builds an in-memory broadcaster with the default waiter caps.
func NewRaceControlBroadcaster() *RaceControlBroadcaster {
	return NewRaceControlBroadcasterWithLimits(DefaultRaceControlMaxWaiters, DefaultRaceControlMaxWaitersPerSession)
}

// NewRaceControlBroadcasterWithLimits builds an in-memory broadcaster that refuses a waiter once
// maxWaiters clients wait in total or maxPerSession wait on the same session.
func NewRaceControlBroadcasterWithLimits(maxWaiters int, maxPerSession int) *RaceControlBroadcaster {
	return &RaceControlBroadcaster{
		subscribers:   make(map[string][]chan []domain.RaceControlEvent),
		maxWaiters:    maxWaiters,
		maxPerSession: maxPerSession,
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
	b.waiters -= len(waiters)
	b.mu.Unlock()

	for _, ch := range waiters {
		ch <- events
		close(ch)
	}
}

// Wait blocks until either a new race control event batch is published for sessionID, the
// context is cancelled, timeout elapses or the broadcaster is closed. The last three return an
// empty result so the caller can respond 200 with `[]` and let the client re-POST, per the
// long-polling contract. It returns domain.ErrRaceControlBusy when a cap is reached and
// domain.ErrShuttingDown after Close, without waiting.
func (b *RaceControlBroadcaster) Wait(ctx context.Context, sessionID string, timeout time.Duration) ([]domain.RaceControlEvent, error) {
	ch := make(chan []domain.RaceControlEvent, 1)

	b.mu.Lock()
	switch {
	case b.closed:
		b.mu.Unlock()
		return nil, domain.ErrShuttingDown
	case b.waiters >= b.maxWaiters, len(b.subscribers[sessionID]) >= b.maxPerSession:
		b.mu.Unlock()
		return nil, domain.ErrRaceControlBusy
	}
	b.subscribers[sessionID] = append(b.subscribers[sessionID], ch)
	b.waiters++
	b.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case events := <-ch:
		return events, nil
	case <-ctx.Done():
	case <-timer.C:
	}
	if !b.removeSubscriber(sessionID, ch) {
		// Publish or Close took this waiter between the select and the removal: the value
		// is already in the buffered channel.
		return <-ch, nil
	}
	return nil, nil
}

// Close releases every waiter with an empty result and refuses new ones. It is registered as a
// server shutdown hook so long polls end at once instead of holding the shutdown for 30 s.
func (b *RaceControlBroadcaster) Close() {
	b.mu.Lock()
	b.closed = true
	all := b.subscribers
	b.subscribers = make(map[string][]chan []domain.RaceControlEvent)
	b.waiters = 0
	b.mu.Unlock()

	for _, waiters := range all {
		for _, ch := range waiters {
			close(ch)
		}
	}
}

// removeSubscriber drops target from the session's waiters. It reports false when target was
// no longer registered (already notified by Publish or Close).
func (b *RaceControlBroadcaster) removeSubscriber(sessionID string, target chan []domain.RaceControlEvent) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	waiters := b.subscribers[sessionID]
	for i, ch := range waiters {
		if ch == target {
			b.subscribers[sessionID] = append(waiters[:i], waiters[i+1:]...)
			if len(b.subscribers[sessionID]) == 0 {
				delete(b.subscribers, sessionID)
			}
			b.waiters--
			return true
		}
	}
	return false
}
