/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live_ingestion.usecase.go - Starts, stops and reports the live ingestion jobs.
##
*/

package usecases

import (
	"context"
	"sync"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
	"overdrive/services/ingestion-service/src/core/ports"
)

// maxFinishedLiveJobs bounds the finished jobs kept for their status.
const maxFinishedLiveJobs = 20

// stopWait is how long Stop waits for the job to end before answering "stopping".
const stopWait = 5 * time.Second

// LiveOptions tunes the live jobs.
type LiveOptions struct {
	// Interval is the base poll interval (see domain.LiveResources).
	Interval time.Duration
	// Grace is how long after the scheduled session end a job keeps polling.
	Grace time.Duration
	// MaxFailures is the number of consecutive failed poll rounds that ends a job, and of failed
	// polls in a row after which a resource is no longer waited for past the session end.
	MaxFailures int
	// MaxJobs is the number of jobs running at once.
	MaxJobs int
}

// LiveIngestionUseCase runs one background job per live session. Jobs live in memory: after a
// restart the job is started again and the rows already stored are not duplicated.
type LiveIngestionUseCase struct {
	provider   ports.OpenF1LiveProvider
	catalog    ports.OpenF1IngestionUseCase
	mapper     ports.ProviderMapper
	dispatcher ports.BatchDispatcher
	guard      *SessionGuard
	options    LiveOptions
	now        func() time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	closed bool
	jobs   map[int]*liveJob
}

// NewLiveIngestionUseCase builds the live use case. catalog ingests the meeting, session and
// drivers; it must not be guarded by guard.
func NewLiveIngestionUseCase(provider ports.OpenF1LiveProvider, catalog ports.OpenF1IngestionUseCase, mapper ports.ProviderMapper, dispatcher ports.BatchDispatcher, guard *SessionGuard, options LiveOptions) *LiveIngestionUseCase {
	ctx, cancel := context.WithCancel(context.Background())
	return &LiveIngestionUseCase{
		provider:   provider,
		catalog:    catalog,
		mapper:     mapper,
		dispatcher: dispatcher,
		guard:      guard,
		options:    options,
		now:        func() time.Time { return time.Now().UTC() },
		ctx:        ctx,
		cancel:     cancel,
		jobs:       map[int]*liveJob{},
	}
}

// Start starts the job of the session. A running job is left alone and its status returned
// (started is false); a finished one is replaced; one being stopped is domain.ErrLiveJobStopping.
func (u *LiveIngestionUseCase) Start(ctx context.Context, sessionKey int) (domain.LiveStatus, bool, error) {
	if status, running, err := runningStatus(u.job(sessionKey)); running {
		return status, false, err
	}
	session, err := u.provider.FindSession(ctx, sessionKey)
	if err != nil {
		return domain.LiveStatus{}, false, err
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return domain.LiveStatus{}, false, domain.ErrLiveShuttingDown
	}
	// Another request may have started it while the session was looked up.
	if status, running, err := runningStatus(u.jobs[sessionKey]); running {
		return status, false, err
	}
	running := 0
	for _, job := range u.jobs {
		if !job.snapshot().State.Final() {
			running++
		}
	}
	if running >= u.options.MaxJobs {
		return domain.LiveStatus{}, false, domain.ErrLiveJobLimit
	}
	if err := u.guard.beginLive(sessionKey); err != nil {
		return domain.LiveStatus{}, false, err
	}

	jobCtx, cancel := context.WithCancel(u.ctx)
	job := newLiveJob(u, session, cancel)
	u.jobs[sessionKey] = job
	u.forgetOldest()
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		defer cancel()
		job.run(jobCtx)
	}()
	return job.snapshot(), true, nil
}

// runningStatus returns the status of a job that is not over; one being stopped cannot be joined
// and is domain.ErrLiveJobStopping.
func runningStatus(job *liveJob) (status domain.LiveStatus, running bool, err error) {
	if job == nil {
		return domain.LiveStatus{}, false, nil
	}
	status = job.snapshot()
	switch {
	case status.State.Final():
		return domain.LiveStatus{}, false, nil
	case status.State == domain.LiveStopping:
		return domain.LiveStatus{}, true, domain.ErrLiveJobStopping
	default:
		return status, true, nil
	}
}

// Stop asks the job to stop and waits for it a few seconds; stopping a finished job is a no-op.
func (u *LiveIngestionUseCase) Stop(ctx context.Context, sessionKey int) (domain.LiveStatus, error) {
	job := u.job(sessionKey)
	if job == nil {
		return domain.LiveStatus{}, domain.ErrLiveJobNotFound
	}
	job.stop()
	timer := time.NewTimer(stopWait)
	defer timer.Stop()
	select {
	case <-job.done:
	case <-timer.C:
	case <-ctx.Done():
	}
	return job.snapshot(), nil
}

// Status reports the latest job of the session.
func (u *LiveIngestionUseCase) Status(sessionKey int) (domain.LiveStatus, error) {
	job := u.job(sessionKey)
	if job == nil {
		return domain.LiveStatus{}, domain.ErrLiveJobNotFound
	}
	return job.snapshot(), nil
}

// Close stops every job and refuses new ones. It does not wait (see Wait).
func (u *LiveIngestionUseCase) Close() {
	u.mu.Lock()
	u.closed = true
	u.mu.Unlock()
	u.cancel()
}

// Wait waits until every job has ended, at most timeout, and reports whether they all did.
func (u *LiveIngestionUseCase) Wait(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		u.wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// job returns the latest job of the session, nil when there is none.
func (u *LiveIngestionUseCase) job(sessionKey int) *liveJob {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.jobs[sessionKey]
}

// forgetOldest drops the job that ended first while too many finished jobs are kept. u.mu held.
func (u *LiveIngestionUseCase) forgetOldest() {
	for {
		finished, oldestKey := 0, 0
		var oldest time.Time
		for key, job := range u.jobs {
			status := job.snapshot()
			if status.EndedAtUTC == nil {
				continue
			}
			finished++
			if oldestKey == 0 || status.EndedAtUTC.Before(oldest) {
				oldestKey, oldest = key, *status.EndedAtUTC
			}
		}
		if finished <= maxFinishedLiveJobs {
			return
		}
		delete(u.jobs, oldestKey)
	}
}
