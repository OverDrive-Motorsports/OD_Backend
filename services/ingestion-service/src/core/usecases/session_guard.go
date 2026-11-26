/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_guard.go - Keeps a manual ingestion and a live job from writing the same session at once.
##
*/

package usecases

import (
	"context"
	"sync"

	"overdrive/services/ingestion-service/src/core/domain"
	"overdrive/services/ingestion-service/src/core/ports"
)

// SessionGuard tracks the sessions whose race data is being written. A manual ingestion replaces
// stored rows while a live job adds some: together they would duplicate samples.
type SessionGuard struct {
	mu     sync.Mutex
	live   map[int]bool
	manual map[int]int
}

// NewSessionGuard builds an empty guard, shared by the manual and live use cases.
func NewSessionGuard() *SessionGuard {
	return &SessionGuard{live: map[int]bool{}, manual: map[int]int{}}
}

// beginLive reserves the session for a live job, or returns domain.ErrSessionBusy.
func (g *SessionGuard) beginLive(sessionKey int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.live[sessionKey] || g.manual[sessionKey] > 0 {
		return domain.ErrSessionBusy
	}
	g.live[sessionKey] = true
	return nil
}

// endLive releases the session of a live job.
func (g *SessionGuard) endLive(sessionKey int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.live, sessionKey)
}

// beginManual reserves the session for a manual ingestion, or returns domain.ErrSessionBusy.
// Several manual ingestions of a session may run at once, as before.
func (g *SessionGuard) beginManual(sessionKey int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.live[sessionKey] {
		return domain.ErrSessionBusy
	}
	g.manual[sessionKey]++
	return nil
}

// endManual releases the session of a manual ingestion.
func (g *SessionGuard) endManual(sessionKey int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.manual[sessionKey]--; g.manual[sessionKey] <= 0 {
		delete(g.manual, sessionKey)
	}
}

// GuardedIngestionUseCase runs manual ingestions under a SessionGuard.
type GuardedIngestionUseCase struct {
	inner ports.OpenF1IngestionUseCase
	guard *SessionGuard
}

// NewGuardedIngestionUseCase wraps the manual ingestion use case.
func NewGuardedIngestionUseCase(inner ports.OpenF1IngestionUseCase, guard *SessionGuard) *GuardedIngestionUseCase {
	return &GuardedIngestionUseCase{inner: inner, guard: guard}
}

// Execute refuses (domain.ErrSessionBusy) an ingestion that would write a resource a live job of
// the same session is adding to. Catalog resources and dry runs are never refused.
func (u *GuardedIngestionUseCase) Execute(ctx context.Context, request domain.OpenF1IngestionRequest) (domain.OpenF1IngestionResult, error) {
	if writesLiveResource(request) {
		if err := u.guard.beginManual(request.SessionKey); err != nil {
			return domain.OpenF1IngestionResult{}, err
		}
		defer u.guard.endManual(request.SessionKey)
	}
	return u.inner.Execute(ctx, request)
}

// writesLiveResource reports whether the request dispatches a resource live jobs write.
func writesLiveResource(request domain.OpenF1IngestionRequest) bool {
	if request.Dispatch != nil && !*request.Dispatch {
		return false
	}
	if len(request.Resources) == 0 {
		return true
	}
	for _, name := range request.Resources {
		for _, resource := range domain.LiveResources {
			if resource.Name == name {
				return true
			}
		}
	}
	return false
}
