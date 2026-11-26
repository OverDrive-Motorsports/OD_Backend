/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live_job.go - One live ingestion job: polls OpenF1 incrementally until the session ends.
##
*/

package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"sync"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

// catalogResources are ingested before polling: the race-data routes need the session to exist in
// championship-service.
var catalogResources = []string{"meetings", "sessions", "drivers"}

// Poll rounds between two attempts to ingest an incomplete catalog, and between two reads of the
// session dates.
const (
	catalogEvery = 20
	sessionEvery = 100
)

// A session running late (red flag) is followed past its grace period while the cars still send
// data: until overrunFreshness without a row, for at most maxOverrun.
const (
	overrunFreshness = 5 * time.Minute
	maxOverrun       = 2 * time.Hour
)

// maxClockSkew bounds how far in the future a row may be dated and still move the cursor: one
// absurd date must not skip the rest of the session.
const maxClockSkew = time.Minute

// livePollTimeout bounds one OpenF1 or catalog call, retries and dispatch included, so a hung
// upstream cannot stall the other resources for the whole dispatch timeout.
const livePollTimeout = 2 * time.Minute

// waitingRefresh is the time between two reads of the dates of a session not started yet. A
// variable so tests can shorten it.
var waitingRefresh = 15 * time.Minute

// sessionPhase is where the session is relative to its scheduled dates.
type sessionPhase int

const (
	phaseUnknown sessionPhase = iota
	phaseBefore
	phaseDuring
	phaseAfter
)

// liveCursor is the incremental state of one resource.
type liveCursor struct {
	resource domain.LiveResource
	// next is where the next poll resumes, before the overlap; zero until a first row or window.
	next time.Time
	// sent holds the fingerprints of the rows of the latest non-empty fetch: the next fetch can
	// only overlap with those.
	sent map[uint64]struct{}
	// failures counts the consecutive failed polls.
	failures int
}

// liveJob follows one session. The session, the phases and the cursors are only used by the job
// goroutine; the status is shared under mu.
type liveJob struct {
	owner   *LiveIngestionUseCase
	session domain.LiveSession
	cancel  context.CancelFunc
	done    chan struct{}
	cursors []*liveCursor
	// catalogPhase is the phase of the session at the latest catalog ingestion: the session
	// status is computed by championship-service when the catalog is stored.
	catalogPhase sessionPhase
	// catalogFailedPhase is the phase whose catalog ingestion failed, retried every catalogEvery.
	catalogFailedPhase sessionPhase

	mu     sync.Mutex
	status domain.LiveStatus
}

// newLiveJob prepares the job of the session; run starts it.
func newLiveJob(owner *LiveIngestionUseCase, session domain.LiveSession, cancel context.CancelFunc) *liveJob {
	job := &liveJob{
		owner:   owner,
		session: session,
		cancel:  cancel,
		done:    make(chan struct{}),
		status: domain.LiveStatus{
			SessionKey:      session.SessionKey,
			SessionID:       contracts.SessionID("openf1", session.SessionKey),
			MeetingKey:      session.MeetingKey,
			State:           domain.LiveRunning,
			SessionStartUTC: session.StartUTC,
			SessionEndUTC:   session.EndUTC,
			StartedAtUTC:    owner.now(),
		},
	}
	if owner.now().Before(session.StartUTC.Add(-domain.LiveLead)) {
		job.status.State = domain.LiveWaiting
	}
	for _, resource := range domain.LiveResources {
		job.cursors = append(job.cursors, &liveCursor{resource: resource})
		job.status.Resources = append(job.status.Resources, domain.LiveResourceStatus{Resource: resource.Name})
	}
	return job
}

// snapshot returns a copy of the status.
func (j *liveJob) snapshot() domain.LiveStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	status := j.status
	status.Resources = append([]domain.LiveResourceStatus{}, j.status.Resources...)
	return status
}

// update changes the status under the lock.
func (j *liveJob) update(change func(status *domain.LiveStatus)) {
	j.mu.Lock()
	defer j.mu.Unlock()
	change(&j.status)
}

// stop interrupts the job, unless it is over.
func (j *liveJob) stop() {
	j.update(func(status *domain.LiveStatus) {
		if !status.State.Final() {
			status.State = domain.LiveStopping
		}
	})
	j.cancel()
}

// phase returns the current phase of the session.
func (j *liveJob) phase() sessionPhase {
	now := j.owner.now()
	switch {
	case now.Before(j.session.StartUTC):
		return phaseBefore
	case now.After(j.session.EndUTC):
		return phaseAfter
	default:
		return phaseDuring
	}
}

// run follows the session until it ends, the context is cancelled or too many rounds fail, then
// releases the session together with the final state.
func (j *liveJob) run(ctx context.Context) {
	state, reason := domain.LiveFailed, "internal error"
	defer func() {
		if recovered := recover(); recovered != nil {
			// A bug in a background job must not take the HTTP service down.
			log.Printf("live ingestion %d: panic: %v", j.session.SessionKey, recovered)
		}
		ended := j.owner.now()
		j.update(func(status *domain.LiveStatus) {
			status.State, status.Error, status.EndedAtUTC = state, reason, &ended
			// Released together with the final state: a client that sees one may rely on the other.
			j.owner.guard.endLive(j.session.SessionKey)
		})
		log.Printf("live ingestion %d: %s %s", j.session.SessionKey, state, reason)
		close(j.done)
	}()
	state, reason = j.follow(ctx)
}

// follow is the job loop; it returns the final state and the reason of a failure.
func (j *liveJob) follow(ctx context.Context) (domain.LiveState, string) {
	if !j.awaitSession(ctx) {
		return domain.LiveStopped, ""
	}
	for _, cursor := range j.cursors {
		if cursor.resource.Window > 0 {
			cursor.next = j.session.StartUTC.Add(-domain.HeavyHistoryLead)
		}
	}
	j.update(func(status *domain.LiveStatus) {
		if status.State == domain.LiveWaiting {
			status.State = domain.LiveRunning
		}
	})

	ticker := time.NewTicker(j.owner.options.Interval)
	defer ticker.Stop()
	for round := 0; ; round++ {
		if round > 0 && round%sessionEvery == 0 {
			j.refreshSession(ctx)
		}
		if j.phase() != j.catalogPhase || round%catalogEvery == 0 {
			j.ingestCatalog(ctx, round%catalogEvery == 0)
		}
		polled, failed, lastReason, refused := 0, 0, "", false
		for index, cursor := range j.cursors {
			if ctx.Err() != nil {
				return domain.LiveStopped, ""
			}
			if round%cursor.resource.Every != 0 {
				continue
			}
			polled++
			if err := j.poll(ctx, index, cursor); err != nil {
				failed++
				lastReason = liveFailure(err)
				refused = refused || errors.Is(err, domain.ErrOpenF1Credentials)
			}
		}
		if ctx.Err() != nil {
			return domain.LiveStopped, ""
		}
		if reason, over := j.countRound(polled, failed, lastReason, refused); over {
			return domain.LiveFailed, reason
		}
		if over, reason := j.sessionOver(); over {
			if failure := j.refresh(ctx); failure != "" {
				reason = failure
			}
			if ctx.Err() != nil {
				return domain.LiveStopped, ""
			}
			if reason != "" {
				return domain.LiveFailed, reason
			}
			return domain.LiveCompleted, ""
		}
		select {
		case <-ctx.Done():
			return domain.LiveStopped, ""
		case <-ticker.C:
		}
	}
}

// awaitSession waits until LiveLead before the session. The dates are read again regularly and
// before leaving: the session may be moved, later or earlier. It returns false when interrupted.
func (j *liveJob) awaitSession(ctx context.Context) bool {
	if j.snapshot().State != domain.LiveWaiting {
		return ctx.Err() == nil
	}
	for {
		opening := j.session.StartUTC.Add(-domain.LiveLead)
		wake := j.owner.now().Add(waitingRefresh)
		if opening.Before(wake) {
			wake = opening
		}
		if !sleepUntil(ctx, j.owner.now, wake) {
			return false
		}
		j.refreshSession(ctx)
		if !j.owner.now().Before(j.session.StartUTC.Add(-domain.LiveLead)) {
			return ctx.Err() == nil
		}
	}
}

// refreshSession reads the session dates again, best effort: on failure the known ones stay.
func (j *liveJob) refreshSession(ctx context.Context) {
	callCtx, cancel := context.WithTimeout(ctx, livePollTimeout)
	defer cancel()
	session, err := j.owner.provider.FindSession(callCtx, j.session.SessionKey)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("live ingestion %d: session dates: %v", j.session.SessionKey, err)
		}
		return
	}
	if session != j.session {
		j.session = session
		// The dates stored by championship-service are stale too.
		j.catalogPhase = phaseUnknown
		j.update(func(status *domain.LiveStatus) {
			status.SessionStartUTC, status.SessionEndUTC = session.StartUTC, session.EndUTC
		})
	}
}

// countRound records the outcome of a poll round: a round where every poll failed counts as one
// failure, any success resets the count. It reports whether the job must give up: at once on
// refused credentials, which will not fix themselves; on any other failure only once the session
// is over, so a short OpenF1 outage during a race does not end live mode.
func (j *liveJob) countRound(polled int, failed int, reason string, refused bool) (string, bool) {
	if polled == 0 {
		return "", false
	}
	over := false
	j.update(func(status *domain.LiveStatus) {
		if failed < polled {
			status.ConsecutiveFailures = 0
			return
		}
		status.ConsecutiveFailures++
		sessionOver := !j.owner.now().Before(j.session.EndUTC.Add(j.owner.options.Grace))
		over = status.ConsecutiveFailures >= j.owner.options.MaxFailures && (refused || sessionOver)
	})
	return reason, over
}

// sessionOver reports whether the grace period has passed and the windowed resources, which may
// still be catching up after a late start, have reached it and the end of the data of a session
// that ran late. A resource that keeps failing is not waited for: the reason then tells the job
// ends incomplete.
func (j *liveJob) sessionOver() (over bool, reason string) {
	end := j.session.EndUTC.Add(j.owner.options.Grace)
	if j.owner.now().Before(end) {
		return false, ""
	}
	resources := j.snapshot().Resources
	for index, cursor := range j.cursors {
		if cursor.failures >= j.owner.options.MaxFailures {
			reason = incompleteReason(resources[index])
			continue
		}
		if cursor.resource.Window <= 0 {
			continue
		}
		if cursor.next.Before(end) {
			return false, ""
		}
		// Measured on the cursor, not on the clock, so a job catching up does not stop early.
		latest := resources[index].LatestRowAtUTC
		if latest != nil && cursor.next.Sub(*latest) < overrunFreshness && cursor.next.Before(end.Add(maxOverrun)) {
			return false, ""
		}
	}
	return true, reason
}

// incompleteReason tells that a resource was still failing when the session ended.
func incompleteReason(resource domain.LiveResourceStatus) string {
	return resource.Resource + " incomplete: " + resource.Error
}

// refresh fetches once more, whole, the resources whose rows may have been completed late. It
// returns the reason of the latest failure, if any.
func (j *liveJob) refresh(ctx context.Context) string {
	reason := ""
	for index, cursor := range j.cursors {
		if cursor.resource.Refresh {
			cursor.next = time.Time{}
			if err := j.poll(ctx, index, cursor); err != nil {
				reason = incompleteReason(j.snapshot().Resources[index])
			}
		}
	}
	return reason
}

// ingestCatalog stores the meeting, session and drivers in championship-service: until the drivers
// (published late by OpenF1) are there too, and again each time the session enters a phase, so its
// status (scheduled, live, finished) follows. Between two retry rounds a phase that could not be
// stored waits for the next one.
func (j *liveJob) ingestCatalog(ctx context.Context, retryRound bool) {
	phase := j.phase()
	if j.snapshot().CatalogIngested && phase == j.catalogPhase {
		return
	}
	if phase == j.catalogFailedPhase && !retryRound {
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, livePollTimeout)
	defer cancel()
	result, err := j.owner.catalog.Execute(callCtx, domain.OpenF1IngestionRequest{
		MeetingKey: j.session.MeetingKey, SessionKey: j.session.SessionKey, Resources: catalogResources,
	})
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		log.Printf("live ingestion %d: catalog: %v", j.session.SessionKey, err)
	}
	complete := err == nil && result.Status == domain.IngestionStatusCompleted
	for _, resource := range result.Resources {
		complete = complete && resource.RowCount > 0
	}
	j.catalogFailedPhase = phase
	if complete {
		j.catalogPhase, j.catalogFailedPhase = phase, phaseUnknown
	}
	j.update(func(status *domain.LiveStatus) {
		status.CatalogIngested = complete
		status.Error = ""
		if !complete {
			status.Error = "catalog not ingested yet"
		}
	})
}

// poll fetches the new rows of one resource, dispatches them, then moves the cursor. On failure
// nothing moves: the next poll covers the same period again.
func (j *liveJob) poll(ctx context.Context, index int, cursor *liveCursor) error {
	window := j.window(cursor)
	pollCtx, cancel := context.WithTimeout(ctx, livePollTimeout)
	defer cancel()
	rows, err := j.owner.provider.FetchWindow(pollCtx, cursor.resource, j.session.SessionKey, window)
	dispatched := 0
	var sent map[uint64]struct{}
	if err == nil {
		var fresh []map[string]any
		fresh, sent = unsentRows(cursor.sent, rows)
		if len(fresh) > 0 {
			dispatched, err = j.dispatch(pollCtx, cursor.resource.Name, fresh)
		}
	}
	if err != nil {
		if ctx.Err() == nil {
			cursor.failures++
			log.Printf("live ingestion %d: %s: %v", j.session.SessionKey, cursor.resource.Name, err)
			j.update(func(status *domain.LiveStatus) { status.Resources[index].Error = liveFailure(err) })
		}
		return err
	}

	cursor.failures = 0
	now := j.owner.now()
	latest, found := latestRow(rows, cursor.resource.DateField, now.Add(maxClockSkew))
	if len(rows) > 0 {
		cursor.sent = sent
	}
	if found && latest.After(cursor.next) {
		cursor.next = latest
	}
	if window.To != nil && window.To.Before(now.Add(-maxClockSkew)) && window.To.After(cursor.next) {
		// The whole window is in the past: nothing more will come for it.
		cursor.next = *window.To
	}
	j.update(func(status *domain.LiveStatus) {
		resource := &status.Resources[index]
		resource.RowCount += dispatched
		resource.Error = ""
		resource.LastSuccessAtUTC = &now
		if found && (resource.LatestRowAtUTC == nil || latest.After(*resource.LatestRowAtUTC)) {
			resource.LatestRowAtUTC = &latest
		}
	})
	return nil
}

// window returns the period of the next poll: from the cursor minus the overlap, up to the
// resource's window.
func (j *liveJob) window(cursor *liveCursor) domain.LiveWindow {
	if cursor.resource.DateField == "" || cursor.next.IsZero() {
		return domain.LiveWindow{}
	}
	from := cursor.next.Add(-cursor.resource.Overlap)
	window := domain.LiveWindow{From: &from}
	if cursor.resource.Window > 0 {
		to := cursor.next.Add(cursor.resource.Window)
		window.To = &to
	}
	return window
}

// dispatch maps the rows and sends them as a merge batch; it returns the number of rows sent.
func (j *liveJob) dispatch(ctx context.Context, resource string, rows []map[string]any) (int, error) {
	dataset, err := j.owner.mapper.Map(resource, rows)
	if err != nil {
		return 0, err
	}
	if len(dataset.Rows) == 0 {
		return 0, nil
	}
	now := j.owner.now()
	_, err = j.owner.dispatcher.Dispatch(ctx, dataset.TargetService, contracts.Batch{
		BatchID:     fmt.Sprintf("openf1-live-%s-%d", resource, now.UnixNano()),
		Provider:    "openf1",
		TriggeredAt: now,
		WriteMode:   contracts.WriteModeMerge,
		Context:     contracts.Context{MeetingKey: j.session.MeetingKey, SessionKey: j.session.SessionKey, Provider: "openf1"},
		Datasets:    []contracts.Dataset{dataset},
		Metadata:    map[string]any{"source": "ingestion-service", "mode": "live"},
	})
	if err != nil {
		return 0, &dispatchError{service: dataset.TargetService, err: err}
	}
	return len(dataset.Rows), nil
}

// dispatchError is a failure to hand a batch to a downstream service.
type dispatchError struct {
	service string
	err     error
}

func (e *dispatchError) Error() string { return "dispatch to " + e.service + ": " + e.err.Error() }
func (e *dispatchError) Unwrap() error { return e.err }

// liveFailure turns a poll error into a short reason, without technical details.
func liveFailure(err error) string {
	var dispatch *dispatchError
	switch {
	case errors.As(err, &dispatch):
		return "dispatch to " + dispatch.service + " failed"
	case errors.Is(err, domain.ErrOpenF1Credentials):
		return "OpenF1 refused the credentials"
	default:
		return fetchFailure(err)
	}
}

// unsentRows returns the rows whose fingerprint is not in sent, and the fingerprints of all rows.
func unsentRows(sent map[uint64]struct{}, rows []map[string]any) ([]map[string]any, map[uint64]struct{}) {
	fresh := make([]map[string]any, 0, len(rows))
	seen := make(map[uint64]struct{}, len(rows))
	for _, row := range rows {
		encoded, err := json.Marshal(row)
		if err != nil {
			fresh = append(fresh, row)
			continue
		}
		hash := fnv.New64a()
		_, _ = hash.Write(encoded)
		key := hash.Sum64()
		seen[key] = struct{}{}
		if _, known := sent[key]; !known {
			fresh = append(fresh, row)
		}
	}
	return fresh, seen
}

// latestRow returns the latest date of the rows, ignoring those after limit.
func latestRow(rows []map[string]any, field string, limit time.Time) (time.Time, bool) {
	var latest time.Time
	for _, row := range rows {
		text, _ := row[field].(string)
		at, err := time.Parse(time.RFC3339, text)
		if err != nil || at.After(limit) {
			continue
		}
		if at.After(latest) {
			latest = at.UTC()
		}
	}
	return latest, !latest.IsZero()
}

// sleepUntil waits for the moment; it returns false when ctx ends first.
func sleepUntil(ctx context.Context, now func() time.Time, moment time.Time) bool {
	wait := moment.Sub(now())
	if wait <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
