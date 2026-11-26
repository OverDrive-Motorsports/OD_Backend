/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live_ingestion_test.go - Unit tests for the live ingestion jobs and the session guard.
##
*/

package usecases

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

// liveStart is the start of the session under test.
var liveStart = time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC)

// fakeLiveProvider is a ports.OpenF1LiveProvider. fetch answers each call; every call is recorded.
type fakeLiveProvider struct {
	mu      sync.Mutex
	session domain.LiveSession
	findErr error
	fetch   func(resource string, window domain.LiveWindow) ([]map[string]any, error)
	calls   map[string][]domain.LiveWindow
}

func (f *fakeLiveProvider) FindSession(ctx context.Context, sessionKey int) (domain.LiveSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	session := f.session
	session.SessionKey = sessionKey
	return session, f.findErr
}

func (f *fakeLiveProvider) FetchWindow(ctx context.Context, resource domain.LiveResource, sessionKey int, window domain.LiveWindow) ([]map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[resource.Name] = append(f.calls[resource.Name], window)
	if f.fetch == nil {
		return nil, nil
	}
	return f.fetch(resource.Name, window)
}

// windows returns the recorded windows of a resource.
func (f *fakeLiveProvider) windows(resource string) []domain.LiveWindow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.LiveWindow{}, f.calls[resource]...)
}

// moveSession changes the dates OpenF1 answers.
func (f *fakeLiveProvider) moveSession(start time.Time, end time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.session.StartUTC, f.session.EndUTC = start, end
}

// fakeLiveDispatcher records the batches and fails with err.
type fakeLiveDispatcher struct {
	mu      sync.Mutex
	err     error
	batches []contracts.Batch
}

func (f *fakeLiveDispatcher) Dispatch(ctx context.Context, service string, batch contracts.Batch) (contracts.DispatchAck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return contracts.DispatchAck{}, f.err
	}
	f.batches = append(f.batches, batch)
	return contracts.DispatchAck{Accepted: true}, nil
}

// setErr changes the dispatch error.
func (f *fakeLiveDispatcher) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// rows counts the dispatched rows of a resource.
func (f *fakeLiveDispatcher) rows(resource string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, batch := range f.batches {
		for _, dataset := range batch.Datasets {
			if dataset.ProviderResource == resource {
				count += len(dataset.Rows)
			}
		}
	}
	return count
}

// liveMapper maps every resource to one race-data dataset, rows unchanged.
type liveMapper struct{}

func (liveMapper) SupportedResources() []string { return nil }

func (liveMapper) Map(resource string, rows []map[string]any) (contracts.Dataset, error) {
	return contracts.Dataset{Name: resource, ProviderResource: resource, TargetService: "race-data-service", RowCount: len(rows), Rows: rows}, nil
}

// fakeCatalog is the manual ingestion used for the catalog.
type fakeCatalog struct {
	mu      sync.Mutex
	calls   []domain.OpenF1IngestionRequest
	drivers []int // row count of the drivers resource per call, the last one repeating
	err     error
	panics  bool
}

func (f *fakeCatalog) Execute(ctx context.Context, request domain.OpenF1IngestionRequest) (domain.OpenF1IngestionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, request)
	if f.panics {
		panic("boom")
	}
	drivers := 20
	if len(f.drivers) > 0 {
		drivers = f.drivers[min(len(f.calls), len(f.drivers))-1]
	}
	return domain.OpenF1IngestionResult{Status: domain.IngestionStatusCompleted, Resources: []domain.ResourceIngestion{
		{Resource: "sessions", RowCount: 1}, {Resource: "drivers", RowCount: drivers},
	}}, f.err
}

// count returns the number of catalog ingestions.
func (f *fakeCatalog) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// liveSetup is a live use case on fakes, at a fixed time the test moves.
type liveSetup struct {
	*LiveIngestionUseCase
	provider   *fakeLiveProvider
	dispatcher *fakeLiveDispatcher
	catalog    *fakeCatalog
	guard      *SessionGuard

	clockMu sync.Mutex
	clock   time.Time
}

// at moves the clock.
func (s *liveSetup) at(moment time.Time) {
	s.clockMu.Lock()
	defer s.clockMu.Unlock()
	s.clock = moment
}

// newLiveSetup builds the use case ten minutes into a two hour session.
func newLiveSetup(t *testing.T) *liveSetup {
	t.Helper()
	s := &liveSetup{
		provider:   &fakeLiveProvider{session: domain.LiveSession{MeetingKey: 1255, StartUTC: liveStart, EndUTC: liveStart.Add(2 * time.Hour)}, calls: map[string][]domain.LiveWindow{}},
		dispatcher: &fakeLiveDispatcher{},
		catalog:    &fakeCatalog{},
		guard:      NewSessionGuard(),
		clock:      liveStart.Add(10 * time.Minute),
	}
	s.LiveIngestionUseCase = NewLiveIngestionUseCase(s.provider, s.catalog, liveMapper{}, s.dispatcher, s.guard,
		LiveOptions{Interval: time.Millisecond, Grace: 30 * time.Minute, MaxFailures: 3, MaxJobs: 1})
	s.now = func() time.Time {
		s.clockMu.Lock()
		defer s.clockMu.Unlock()
		return s.clock
	}
	t.Cleanup(func() {
		s.Close()
		if !s.Wait(5 * time.Second) {
			t.Error("live jobs did not end")
		}
	})
	return s
}

// start starts the job of session 9998.
func (s *liveSetup) start(t *testing.T) domain.LiveStatus {
	t.Helper()
	status, started, err := s.Start(context.Background(), 9998)
	if err != nil || !started {
		t.Fatalf("Start: started=%v err=%v", started, err)
	}
	return status
}

// await polls the condition until it holds.
func await(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// awaitState waits for the job of session 9998 to reach the state.
func (s *liveSetup) awaitState(t *testing.T, state domain.LiveState) domain.LiveStatus {
	t.Helper()
	var status domain.LiveStatus
	await(t, string(state), func() bool {
		status, _ = s.Status(9998)
		return status.State == state
	})
	return status
}

// awaitRounds waits until position was polled n more times.
func (s *liveSetup) awaitRounds(t *testing.T, n int) {
	t.Helper()
	polls := len(s.provider.windows("position"))
	await(t, "more rounds", func() bool { return len(s.provider.windows("position")) >= polls+n })
}

// liveSample is a provider row dated at the given offset of the session.
func liveSample(offset time.Duration) map[string]any {
	return map[string]any{"driver_number": float64(44), "date": liveStart.Add(offset).Format(time.RFC3339Nano)}
}

// resourceStatus returns one resource of a status.
func resourceStatus(status domain.LiveStatus, name string) domain.LiveResourceStatus {
	for _, resource := range status.Resources {
		if resource.Resource == name {
			return resource
		}
	}
	return domain.LiveResourceStatus{}
}

// TestLive_Incremental proves the first poll of a light resource is open, the next ones start at
// the latest known row minus the overlap, a row already sent is not sent again, and the batches
// are merge batches carrying the session.
func TestLive_Incremental(t *testing.T) {
	s := newLiveSetup(t)
	s.provider.fetch = func(resource string, window domain.LiveWindow) ([]map[string]any, error) {
		if resource != "position" {
			return nil, nil
		}
		rows := []map[string]any{liveSample(5 * time.Minute)}
		if window.From != nil {
			rows = append(rows, liveSample(6*time.Minute))
		}
		return rows, nil
	}
	status := s.start(t)
	if status.State != domain.LiveRunning || status.SessionID != "openf1:session:9998" || status.MeetingKey != 1255 || status.EndedAtUTC != nil {
		t.Fatalf("status = %+v", status)
	}
	await(t, "four position polls", func() bool { return len(s.provider.windows("position")) >= 4 })

	windows := s.provider.windows("position")
	if windows[0].From != nil || windows[0].To != nil {
		t.Fatalf("first window = %+v, want open", windows[0])
	}
	if want := liveStart.Add(5*time.Minute - 10*time.Second); windows[1].From == nil || !windows[1].From.Equal(want) || windows[1].To != nil {
		t.Fatalf("second window = %+v, want from %s", windows[1], want)
	}
	if want := liveStart.Add(6*time.Minute - 10*time.Second); !windows[2].From.Equal(want) {
		t.Fatalf("third window = %+v, want from %s", windows[2], want)
	}
	stopped, err := s.Stop(context.Background(), 9998)
	if err != nil || stopped.State != domain.LiveStopped || stopped.EndedAtUTC == nil {
		t.Fatalf("Stop: %+v err=%v", stopped, err)
	}
	if got := s.dispatcher.rows("position"); got != 2 {
		t.Fatalf("%d position rows dispatched, want 2", got)
	}
	position := resourceStatus(stopped, "position")
	if position.RowCount != 2 || position.LatestRowAtUTC == nil || !position.LatestRowAtUTC.Equal(liveStart.Add(6*time.Minute)) || position.LastSuccessAtUTC == nil {
		t.Fatalf("position status = %+v", position)
	}
	batch := s.dispatcher.batches[0]
	if batch.WriteMode != contracts.WriteModeMerge || batch.Provider != "openf1" || batch.Context.SessionKey != 9998 || batch.Context.MeetingKey != 1255 || batch.BatchID == "" {
		t.Fatalf("batch = %+v", batch)
	}
}

// TestLive_ChangedRowIsSentAgain proves a row whose content changed (a stint getting its last lap)
// is dispatched again while an identical one is not.
func TestLive_ChangedRowIsSentAgain(t *testing.T) {
	fresh, sent := unsentRows(map[uint64]struct{}{}, []map[string]any{{"stint_number": 1, "lap_end": 10}, {"stint_number": 2}})
	if len(fresh) != 2 {
		t.Fatalf("fresh = %v", fresh)
	}
	fresh, _ = unsentRows(sent, []map[string]any{{"stint_number": 1, "lap_end": 12}, {"stint_number": 2}})
	if len(fresh) != 1 || fresh[0]["lap_end"] != 12 {
		t.Fatalf("fresh = %v", fresh)
	}
}

// TestLive_HeavyResourcesAreWindowed proves car_data starts an hour before the session (the laps to
// the grid), in bounded windows, and skips an empty past window instead of asking for it forever.
func TestLive_HeavyResourcesAreWindowed(t *testing.T) {
	s := newLiveSetup(t)
	s.start(t)
	await(t, "three car_data polls", func() bool { return len(s.provider.windows("car_data")) >= 3 })
	for index, window := range s.provider.windows("car_data")[:3] {
		base := liveStart.Add(-domain.HeavyHistoryLead).Add(time.Duration(index) * 5 * time.Minute)
		if !window.From.Equal(base.Add(-10*time.Second)) || !window.To.Equal(base.Add(5*time.Minute)) {
			t.Fatalf("window %d = %s..%s", index, window.From, window.To)
		}
	}
}

// TestLive_HeavyWindowWaitsForThePresent proves the cursor never passes a window that is not over:
// its rows may still be coming, so the windows stop at the present and are asked for again.
func TestLive_HeavyWindowWaitsForThePresent(t *testing.T) {
	s := newLiveSetup(t)
	now := liveStart.Add(-domain.LiveLead).Add(5 * time.Minute)
	s.at(now)
	s.start(t)
	await(t, "the catch up", func() bool {
		windows := s.provider.windows("car_data")
		return len(windows) >= 3 && windows[len(windows)-1].To.Equal(*windows[len(windows)-2].To)
	})
	for _, window := range s.provider.windows("car_data") {
		if window.To.After(now) {
			t.Fatalf("window up to %s, after the present %s", window.To, now)
		}
	}
}

// TestLive_FutureDateDoesNotMoveTheCursor proves a row dated far in the future is stored but does
// not make the next polls skip the rest of the session.
func TestLive_FutureDateDoesNotMoveTheCursor(t *testing.T) {
	latest, found := latestRow([]map[string]any{liveSample(time.Minute), liveSample(48 * time.Hour), {"date": 12}, {"date": "soon"}}, "date", liveStart.Add(time.Hour))
	if !found || !latest.Equal(liveStart.Add(time.Minute)) {
		t.Fatalf("latest = %s %v", latest, found)
	}
	if _, found := latestRow([]map[string]any{{"stint_number": 1}}, "", liveStart); found {
		t.Fatal("no date expected")
	}
}

// TestLive_DispatchFailureIsRetried proves a failed dispatch moves nothing: the same rows are sent
// once the downstream service is back, and the error is cleared.
func TestLive_DispatchFailureIsRetried(t *testing.T) {
	s := newLiveSetup(t)
	s.options.MaxFailures = 1 << 30
	s.dispatcher.setErr(errors.New("dial tcp 10.0.0.1: refused"))
	s.provider.fetch = func(resource string, window domain.LiveWindow) ([]map[string]any, error) {
		if resource == "position" {
			return []map[string]any{liveSample(time.Minute)}, nil
		}
		return nil, nil
	}
	s.start(t)
	await(t, "the dispatch error", func() bool {
		status, _ := s.Status(9998)
		return resourceStatus(status, "position").Error == "dispatch to race-data-service failed"
	})
	s.dispatcher.setErr(nil)
	await(t, "the retry", func() bool {
		status, _ := s.Status(9998)
		position := resourceStatus(status, "position")
		return position.Error == "" && position.RowCount == 1
	})
	for _, window := range s.provider.windows("position")[:2] {
		if window.From != nil {
			t.Fatalf("the cursor moved on a failed dispatch: %+v", window)
		}
	}
}

// TestLive_GivesUpAfterConsecutiveFailures proves refused credentials end the job even during the
// session, with a short reason, and release the session with the final state.
func TestLive_GivesUpAfterConsecutiveFailures(t *testing.T) {
	s := newLiveSetup(t)
	s.provider.fetch = func(string, domain.LiveWindow) ([]map[string]any, error) {
		return nil, fmt.Errorf("%w: body s3cret", domain.ErrOpenF1Credentials)
	}
	s.start(t)
	status := s.awaitState(t, domain.LiveFailed)
	if status.ConsecutiveFailures != 3 || status.Error != "OpenF1 refused the credentials" || status.EndedAtUTC == nil {
		t.Fatalf("status = %+v", status)
	}
	if err := s.guard.beginManual(9998); err != nil {
		t.Fatalf("the session is not released with the final state: %v", err)
	}
}

// TestLive_OutageDuringTheSessionKeepsTheJob proves an OpenF1 outage during the session does not
// end the job: failures are counted and reported, and polling resumes when OpenF1 is back.
func TestLive_OutageDuringTheSessionKeepsTheJob(t *testing.T) {
	s := newLiveSetup(t)
	var mu sync.Mutex
	down := true
	s.provider.fetch = func(string, domain.LiveWindow) ([]map[string]any, error) {
		mu.Lock()
		defer mu.Unlock()
		if down {
			return nil, errors.New("upstream status 502")
		}
		return nil, nil
	}
	s.start(t)
	await(t, "many failed rounds", func() bool {
		status, _ := s.Status(9998)
		return status.ConsecutiveFailures >= 10
	})
	if status, _ := s.Status(9998); status.State != domain.LiveRunning {
		t.Fatalf("state = %s during an outage", status.State)
	}
	mu.Lock()
	down = false
	mu.Unlock()
	await(t, "the recovery", func() bool {
		status, _ := s.Status(9998)
		return status.ConsecutiveFailures == 0
	})
}

// TestLive_OutageAfterTheSessionEndsTheJob proves the same outage ends a job once the session is
// over, instead of polling forever.
func TestLive_OutageAfterTheSessionEndsTheJob(t *testing.T) {
	s := newLiveSetup(t)
	s.at(liveStart.Add(2*time.Hour + 31*time.Minute))
	s.provider.fetch = func(string, domain.LiveWindow) ([]map[string]any, error) {
		return nil, errors.New("upstream status 502")
	}
	s.start(t)
	if status := s.awaitState(t, domain.LiveFailed); status.Error == "" {
		t.Fatalf("status = %+v", status)
	}
}

// TestLive_OneWorkingResourceKeepsTheJob proves a single failing resource is reported without ending
// the job.
func TestLive_OneWorkingResourceKeepsTheJob(t *testing.T) {
	s := newLiveSetup(t)
	s.provider.fetch = func(resource string, window domain.LiveWindow) ([]map[string]any, error) {
		if resource == "race_control" {
			return nil, errors.New("boom")
		}
		return nil, nil
	}
	s.start(t)
	await(t, "ten rounds", func() bool { return len(s.provider.windows("position")) >= 10 })
	status, _ := s.Status(9998)
	if status.State != domain.LiveRunning || status.ConsecutiveFailures != 0 || resourceStatus(status, "race_control").Error != "OpenF1 request failed" {
		t.Fatalf("status = %+v", status)
	}
}

// TestLive_CompletesAfterGrace proves the job ends by itself once the grace period has passed, not
// before, after a last whole fetch of the resources completed late.
func TestLive_CompletesAfterGrace(t *testing.T) {
	s := newLiveSetup(t)
	s.at(liveStart.Add(2*time.Hour + 29*time.Minute))
	s.start(t)
	s.awaitRounds(t, 5)
	if status, _ := s.Status(9998); status.State != domain.LiveRunning {
		t.Fatalf("state = %s during the grace period", status.State)
	}
	s.at(liveStart.Add(2*time.Hour + 32*time.Minute))
	status := s.awaitState(t, domain.LiveCompleted)
	if status.Error != "" || status.EndedAtUTC == nil {
		t.Fatalf("status = %+v", status)
	}
	laps := s.provider.windows("laps")
	if last := laps[len(laps)-1]; last.From != nil {
		t.Fatalf("last laps fetch = %+v, want the whole resource", last)
	}
}

// TestLive_FinishedSessionIsCaughtUp proves a job started on a past session covers the whole session
// window by window before it completes.
func TestLive_FinishedSessionIsCaughtUp(t *testing.T) {
	s := newLiveSetup(t)
	s.at(liveStart.Add(240 * time.Hour))
	s.start(t)
	s.awaitState(t, domain.LiveCompleted)
	windows := s.provider.windows("location")
	if last := windows[len(windows)-1].To; last.Before(liveStart.Add(2*time.Hour + 30*time.Minute)) {
		t.Fatalf("last window ends at %s", last)
	}
	if len(windows) != 42 {
		t.Fatalf("%d windows of 5 minutes for 3 h 30, want 42", len(windows))
	}
}

// TestLive_FollowsALateSession proves a session still sending data past its grace period is followed
// until the data stops.
func TestLive_FollowsALateSession(t *testing.T) {
	s := newLiveSetup(t)
	latest := 2*time.Hour + 28*time.Minute
	s.at(liveStart.Add(latest))
	s.provider.fetch = func(resource string, window domain.LiveWindow) ([]map[string]any, error) {
		if resource == "location" && !window.To.Before(liveStart.Add(latest)) {
			return []map[string]any{liveSample(latest)}, nil
		}
		return nil, nil
	}
	s.start(t)
	await(t, "the catch up", func() bool {
		status, _ := s.Status(9998)
		return resourceStatus(status, "location").LatestRowAtUTC != nil
	})
	s.at(liveStart.Add(2*time.Hour + 32*time.Minute))
	s.awaitRounds(t, 5)
	if status, _ := s.Status(9998); status.State != domain.LiveRunning {
		t.Fatalf("state = %s while the cars still send data", status.State)
	}
	s.at(liveStart.Add(2*time.Hour + 35*time.Minute))
	s.awaitState(t, domain.LiveCompleted)
}

// TestLive_CatchUpCoversALateSession proves a job that is not up to date (started late, or after a
// restart) follows the rows past the grace period instead of completing without them.
func TestLive_CatchUpCoversALateSession(t *testing.T) {
	s := newLiveSetup(t)
	s.at(liveStart.Add(240 * time.Hour))
	last := liveStart.Add(3*time.Hour + 30*time.Minute)
	s.provider.fetch = func(resource string, window domain.LiveWindow) ([]map[string]any, error) {
		if resource != "location" {
			return nil, nil
		}
		at := *window.To
		if at.After(last) {
			at = last
		}
		if at.Before(*window.From) {
			return nil, nil
		}
		return []map[string]any{liveSample(at.Sub(liveStart))}, nil
	}
	s.start(t)
	status := s.awaitState(t, domain.LiveCompleted)
	if latest := resourceStatus(status, "location").LatestRowAtUTC; latest == nil || !latest.Equal(last) {
		t.Fatalf("latest = %v, want %s", latest, last)
	}
}

// TestLive_OverrunIsCapped proves a resource sending data forever does not hold the job beyond
// maxOverrun.
func TestLive_OverrunIsCapped(t *testing.T) {
	s := newLiveSetup(t)
	s.at(liveStart.Add(240 * time.Hour))
	s.provider.fetch = func(resource string, window domain.LiveWindow) ([]map[string]any, error) {
		if resource != "location" {
			return nil, nil
		}
		return []map[string]any{liveSample(window.To.Sub(liveStart))}, nil
	}
	s.start(t)
	status := s.awaitState(t, domain.LiveCompleted)
	if latest := resourceStatus(status, "location").LatestRowAtUTC; latest.After(liveStart.Add(2*time.Hour + 30*time.Minute + maxOverrun + 10*time.Minute)) {
		t.Fatalf("latest = %s", latest)
	}
}

// TestLive_FailingResourceDoesNotBlockTheEnd proves a windowed resource that always fails does not
// keep the job (and the session) forever: past the end it finishes as failed, naming the resource.
func TestLive_FailingResourceDoesNotBlockTheEnd(t *testing.T) {
	s := newLiveSetup(t)
	s.at(liveStart.Add(240 * time.Hour))
	s.provider.fetch = func(resource string, window domain.LiveWindow) ([]map[string]any, error) {
		if resource == "location" {
			return nil, errors.New("boom")
		}
		return nil, nil
	}
	s.start(t)
	if status := s.awaitState(t, domain.LiveFailed); status.Error != "location incomplete: OpenF1 request failed" {
		t.Fatalf("status = %+v", status)
	}
	if err := s.guard.beginManual(9998); err != nil {
		t.Fatalf("the session is not released with the final state: %v", err)
	}
}

// TestLive_AnyFailingResourceIsReportedAtTheEnd proves the job does not claim to be complete when a
// light resource never worked, or when the last whole fetch fails.
func TestLive_AnyFailingResourceIsReportedAtTheEnd(t *testing.T) {
	for _, failing := range []string{"race_control", "laps"} {
		s := newLiveSetup(t)
		s.at(liveStart.Add(240 * time.Hour))
		s.provider.fetch = func(resource string, window domain.LiveWindow) ([]map[string]any, error) {
			if resource == failing {
				return nil, errors.New("boom")
			}
			return nil, nil
		}
		s.start(t)
		if status := s.awaitState(t, domain.LiveFailed); status.Error != failing+" incomplete: OpenF1 request failed" {
			t.Fatalf("%s: status = %+v", failing, status)
		}
	}
}

// TestLive_WaitsForTheSession proves a job started early polls nothing, and can be stopped.
func TestLive_WaitsForTheSession(t *testing.T) {
	s := newLiveSetup(t)
	s.at(liveStart.Add(-2 * time.Hour))
	if status := s.start(t); status.State != domain.LiveWaiting {
		t.Fatalf("state = %s", status.State)
	}
	time.Sleep(20 * time.Millisecond)
	if len(s.provider.windows("position")) != 0 {
		t.Fatal("polled before the session")
	}
	if status, err := s.Stop(context.Background(), 9998); err != nil || status.State != domain.LiveStopped {
		t.Fatalf("Stop: %+v err=%v", status, err)
	}
}

// TestLive_RescheduledWhileWaiting proves a session postponed while the job waits is waited for again.
func TestLive_RescheduledWhileWaiting(t *testing.T) {
	s := newLiveSetup(t)
	opening := liveStart.Add(-domain.LiveLead)
	s.at(opening.Add(-30 * time.Millisecond))
	if status := s.start(t); status.State != domain.LiveWaiting {
		t.Fatalf("state = %s", status.State)
	}
	s.provider.moveSession(liveStart.Add(24*time.Hour), liveStart.Add(26*time.Hour))
	s.at(opening)
	await(t, "the new dates", func() bool {
		status, _ := s.Status(9998)
		return status.SessionStartUTC.Equal(liveStart.Add(24 * time.Hour))
	})
	time.Sleep(20 * time.Millisecond)
	if status, _ := s.Status(9998); status.State != domain.LiveWaiting || len(s.provider.windows("position")) != 0 {
		t.Fatalf("state = %s, polls = %d", status.State, len(s.provider.windows("position")))
	}
}

// TestLive_BroughtForwardWhileWaiting proves a session moved earlier is noticed while waiting, not
// at its former date.
func TestLive_BroughtForwardWhileWaiting(t *testing.T) {
	previous := waitingRefresh
	waitingRefresh = 5 * time.Millisecond
	defer func() { waitingRefresh = previous }()
	s := newLiveSetup(t)
	s.at(liveStart.Add(-10 * time.Hour))
	s.start(t)
	s.provider.moveSession(liveStart.Add(-9*time.Hour-45*time.Minute), liveStart.Add(-8*time.Hour))
	s.awaitState(t, domain.LiveRunning)
	if _, err := s.Stop(context.Background(), 9998); err != nil {
		t.Fatal(err)
	}
}

// TestLive_SessionDatesAreRefreshed proves a session moved while the job runs is followed on its new
// dates.
func TestLive_SessionDatesAreRefreshed(t *testing.T) {
	s := newLiveSetup(t)
	s.start(t)
	moved := liveStart.Add(5 * time.Hour)
	s.provider.moveSession(liveStart, moved)
	await(t, "the new dates", func() bool {
		status, _ := s.Status(9998)
		return status.SessionEndUTC.Equal(moved)
	})
	s.at(liveStart.Add(3 * time.Hour))
	s.awaitRounds(t, 5)
	if status, _ := s.Status(9998); status.State != domain.LiveRunning {
		t.Fatalf("state = %s", status.State)
	}
}

// TestLive_CatalogUntilDrivers proves the catalog is ingested first, again while OpenF1 has no
// drivers yet, then not again within the same phase.
func TestLive_CatalogUntilDrivers(t *testing.T) {
	s := newLiveSetup(t)
	s.catalog.drivers = []int{0, 20}
	s.start(t)
	await(t, "the catalog", func() bool {
		status, _ := s.Status(9998)
		return status.CatalogIngested && status.Error == ""
	})
	s.awaitRounds(t, 25)
	s.catalog.mu.Lock()
	defer s.catalog.mu.Unlock()
	if len(s.catalog.calls) != 2 || s.catalog.calls[0].MeetingKey != 1255 || s.catalog.calls[0].SessionKey != 9998 || len(s.catalog.calls[0].Resources) != 3 {
		t.Fatalf("catalog calls = %+v", s.catalog.calls)
	}
}

// TestLive_CatalogFollowsThePhases proves the catalog is stored again when the session starts and
// ends, so its status never stays scheduled or live, and not in between.
func TestLive_CatalogFollowsThePhases(t *testing.T) {
	s := newLiveSetup(t)
	s.at(liveStart.Add(-10 * time.Minute))
	s.start(t)
	await(t, "the first catalog", func() bool { return s.catalog.count() == 1 })
	s.at(liveStart.Add(time.Minute))
	await(t, "the catalog at the start", func() bool { return s.catalog.count() == 2 })
	s.awaitRounds(t, 25)
	if got := s.catalog.count(); got != 2 {
		t.Fatalf("%d catalog ingestions during the session, want 2", got)
	}
	s.at(liveStart.Add(2*time.Hour + time.Minute))
	await(t, "the catalog at the end", func() bool { return s.catalog.count() == 3 })
}

// TestLive_CatalogFailureIsReported proves a failing catalog neither stops the polls nor leaks its
// cause.
func TestLive_CatalogFailureIsReported(t *testing.T) {
	s := newLiveSetup(t)
	s.catalog.err = errors.New("dial tcp 10.0.0.1")
	s.start(t)
	s.awaitRounds(t, 2)
	if status, _ := s.Status(9998); status.CatalogIngested || status.Error != "catalog not ingested yet" {
		t.Fatalf("status = %+v", status)
	}
}

// TestLive_StartIsIdempotent proves starting a running job, even from concurrent requests, returns
// its status without a second job, and a finished job can be started again.
func TestLive_StartIsIdempotent(t *testing.T) {
	s := newLiveSetup(t)
	var wg sync.WaitGroup
	results := make(chan bool, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, started, err := s.Start(context.Background(), 9998)
			if err != nil {
				t.Errorf("Start: %v", err)
			}
			results <- started
		}()
	}
	wg.Wait()
	close(results)
	count := 0
	for started := range results {
		if started {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("%d jobs started", count)
	}
	if _, err := s.Stop(context.Background(), 9998); err != nil {
		t.Fatal(err)
	}
	s.start(t)
}

// TestLive_StartWhileStopping proves a job being stopped cannot be joined: the caller must not
// believe the session is followed.
func TestLive_StartWhileStopping(t *testing.T) {
	s := newLiveSetup(t)
	s.start(t)
	s.job(9998).update(func(status *domain.LiveStatus) { status.State = domain.LiveStopping })
	if _, _, err := s.Start(context.Background(), 9998); !errors.Is(err, domain.ErrLiveJobStopping) {
		t.Fatalf("err = %v", err)
	}
}

// TestLive_StartRefusals proves the refusals: unknown session, too many jobs, session being ingested
// by hand, service shutting down. None leaves a job behind.
func TestLive_StartRefusals(t *testing.T) {
	s := newLiveSetup(t)
	s.provider.findErr = domain.ErrLiveSessionNotFound
	if _, _, err := s.Start(context.Background(), 1); !errors.Is(err, domain.ErrLiveSessionNotFound) {
		t.Fatalf("err = %v", err)
	}
	s.provider.findErr = nil
	if err := s.guard.beginManual(1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Start(context.Background(), 1); !errors.Is(err, domain.ErrSessionBusy) {
		t.Fatalf("err = %v", err)
	}
	s.guard.endManual(1)
	if _, err := s.Status(1); !errors.Is(err, domain.ErrLiveJobNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.Stop(context.Background(), 1); !errors.Is(err, domain.ErrLiveJobNotFound) {
		t.Fatalf("err = %v", err)
	}
	s.start(t)
	if _, _, err := s.Start(context.Background(), 2); !errors.Is(err, domain.ErrLiveJobLimit) {
		t.Fatalf("err = %v", err)
	}
	s.Close()
	s.awaitState(t, domain.LiveStopped)
	if _, _, err := s.Start(context.Background(), 2); !errors.Is(err, domain.ErrLiveShuttingDown) {
		t.Fatalf("err = %v", err)
	}
}

// TestLive_PanicEndsOnlyTheJob proves a panic in a job is contained: the job fails, the session is
// released and the process goes on.
func TestLive_PanicEndsOnlyTheJob(t *testing.T) {
	s := newLiveSetup(t)
	s.catalog.panics = true
	s.start(t)
	if status := s.awaitState(t, domain.LiveFailed); status.Error != "internal error" {
		t.Fatalf("status = %+v", status)
	}
	if err := s.guard.beginLive(9998); err != nil {
		t.Fatalf("the session is not released: %v", err)
	}
}

// TestLive_FinishedJobsAreBounded proves only the latest finished jobs are kept.
func TestLive_FinishedJobsAreBounded(t *testing.T) {
	s := newLiveSetup(t)
	for key := 1; key <= maxFinishedLiveJobs+5; key++ {
		s.at(liveStart.Add(time.Duration(key) * time.Second))
		if _, _, err := s.Start(context.Background(), key); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Stop(context.Background(), key); err != nil {
			t.Fatal(err)
		}
	}
	s.LiveIngestionUseCase.mu.Lock()
	defer s.LiveIngestionUseCase.mu.Unlock()
	if len(s.jobs) > maxFinishedLiveJobs+1 || s.jobs[1] != nil || s.jobs[maxFinishedLiveJobs+5] == nil {
		t.Fatalf("%d jobs kept", len(s.jobs))
	}
}

// TestLiveResources_RateBudget proves the schedule stays under the 60 calls a minute of an OpenF1
// subscription with the default interval.
func TestLiveResources_RateBudget(t *testing.T) {
	calls := 0.0
	for _, resource := range domain.LiveResources {
		calls += 20 / float64(resource.Every)
	}
	if calls > 55 {
		t.Fatalf("%.0f calls a minute", calls)
	}
}

// countingIngestion records the manual ingestions that reach the inner use case.
type countingIngestion struct {
	calls  int
	during func()
}

func (c *countingIngestion) Execute(ctx context.Context, request domain.OpenF1IngestionRequest) (domain.OpenF1IngestionResult, error) {
	c.calls++
	if c.during != nil {
		c.during()
	}
	return domain.OpenF1IngestionResult{}, nil
}

// TestGuardedIngestion proves a manual ingestion of race data is refused during a live job of the
// same session only, catalog resources and dry runs are not, and a live job cannot start while
// a manual ingestion runs.
func TestGuardedIngestion(t *testing.T) {
	guard := NewSessionGuard()
	inner := &countingIngestion{}
	usecase := NewGuardedIngestionUseCase(inner, guard)
	if err := guard.beginLive(1); err != nil {
		t.Fatal(err)
	}
	dryRun := false
	for _, tc := range []struct {
		name    string
		request domain.OpenF1IngestionRequest
		refused bool
	}{
		{"every resource", domain.OpenF1IngestionRequest{SessionKey: 1}, true},
		{"a live resource", domain.OpenF1IngestionRequest{SessionKey: 1, Resources: []string{"drivers", "laps"}}, true},
		{"catalog only", domain.OpenF1IngestionRequest{SessionKey: 1, Resources: []string{"session_result"}}, false},
		{"dry run", domain.OpenF1IngestionRequest{SessionKey: 1, Dispatch: &dryRun}, false},
		{"another session", domain.OpenF1IngestionRequest{SessionKey: 2}, false},
	} {
		before := inner.calls
		_, err := usecase.Execute(context.Background(), tc.request)
		if errors.Is(err, domain.ErrSessionBusy) != tc.refused || (inner.calls == before) != tc.refused {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
	}
	guard.endLive(1)
	inner.during = func() {
		if err := guard.beginLive(1); !errors.Is(err, domain.ErrSessionBusy) {
			t.Errorf("live during manual: %v", err)
		}
	}
	if _, err := usecase.Execute(context.Background(), domain.OpenF1IngestionRequest{SessionKey: 1}); err != nil {
		t.Fatal(err)
	}
	if err := guard.beginLive(1); err != nil {
		t.Fatalf("live after manual: %v", err)
	}
}
