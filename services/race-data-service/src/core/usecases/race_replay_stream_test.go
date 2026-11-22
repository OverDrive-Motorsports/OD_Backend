/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_replay_stream_test.go - Package usecases source file for services/race-data-service/src/core/usecases.
##
*/

package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// fakeRaceStreamRepository is an in-memory ports.RaceStreamRepository: every driver-scoped
// method filters its rows by the requested driver, like the Prisma repository, and records the
// calls so tests can tell whether the sample tables were read.
type fakeRaceStreamRepository struct {
	mu          sync.Mutex
	drivers     []int
	telemetry   []domain.RaceReplayTelemetryFrame
	positions   []domain.RacePosition
	raceControl []domain.RaceControlEvent
	weather     []domain.RaceWeatherSample
	radio       []domain.RaceRadioMessage
	pitStops    []ports.RaceReplayPitStopFrame
	stints      []ports.RaceReplayStintFrame
	laps        []ports.RaceReplayLapFrame
	err         error
	calls       int
	driverCalls int
	windows     []domain.TimeWindow
}

func (f *fakeRaceStreamRepository) record(window domain.TimeWindow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.windows = append(f.windows, window)
	return f.err
}

func (f *fakeRaceStreamRepository) ListReplayDriverNumbers(ctx context.Context, sessionID string) ([]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.driverCalls++
	return append([]int(nil), f.drivers...), f.err
}

func (f *fakeRaceStreamRepository) ListTelemetryFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]domain.RaceReplayTelemetryFrame, error) {
	if err := f.record(window); err != nil {
		return nil, err
	}
	var out []domain.RaceReplayTelemetryFrame
	for _, frame := range f.telemetry {
		if frame.DriverNumber == driverNumber {
			out = append(out, frame)
		}
	}
	return out, nil
}

func (f *fakeRaceStreamRepository) ListPositionFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]domain.RacePosition, error) {
	if err := f.record(window); err != nil {
		return nil, err
	}
	var out []domain.RacePosition
	for _, item := range f.positions {
		if item.DriverNumber == driverNumber {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeRaceStreamRepository) ListRaceControlFrames(ctx context.Context, sessionID string, window domain.TimeWindow) ([]domain.RaceControlEvent, error) {
	return f.raceControl, f.record(window)
}

func (f *fakeRaceStreamRepository) ListWeatherFrames(ctx context.Context, sessionID string, window domain.TimeWindow) ([]domain.RaceWeatherSample, error) {
	return f.weather, f.record(window)
}

func (f *fakeRaceStreamRepository) ListRadioFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]domain.RaceRadioMessage, error) {
	if err := f.record(window); err != nil {
		return nil, err
	}
	var out []domain.RaceRadioMessage
	for _, item := range f.radio {
		if item.DriverNumber == driverNumber {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeRaceStreamRepository) ListPitStopFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]ports.RaceReplayPitStopFrame, error) {
	if err := f.record(window); err != nil {
		return nil, err
	}
	var out []ports.RaceReplayPitStopFrame
	for _, item := range f.pitStops {
		if item.Payload.DriverNumber == driverNumber {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeRaceStreamRepository) ListStintFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]ports.RaceReplayStintFrame, error) {
	if err := f.record(window); err != nil {
		return nil, err
	}
	var out []ports.RaceReplayStintFrame
	for _, item := range f.stints {
		if item.Payload.DriverNumber == driverNumber {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeRaceStreamRepository) ListLapFrames(ctx context.Context, sessionID string, driverNumber int, window domain.TimeWindow) ([]ports.RaceReplayLapFrame, error) {
	if err := f.record(window); err != nil {
		return nil, err
	}
	var out []ports.RaceReplayLapFrame
	for _, item := range f.laps {
		if item.Payload.DriverNumber == driverNumber {
			out = append(out, item)
		}
	}
	return out, nil
}

// fakeReplayCache is an in-memory ports.ReplayCache.
type fakeReplayCache struct {
	mu          sync.Mutex
	archives    map[string]*fakeArchive
	nextID      int
	findErr     error
	saveErr     error
	invalidated []string
}

type fakeArchive struct {
	sessionID string
	complete  bool
	parts     []domain.ReplayPart
}

func newFakeReplayCache() *fakeReplayCache {
	return &fakeReplayCache{archives: map[string]*fakeArchive{}}
}

func (f *fakeReplayCache) FindComplete(ctx context.Context, sessionID string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.findErr != nil {
		return "", false, f.findErr
	}
	for id, archive := range f.archives {
		if archive.sessionID == sessionID && archive.complete {
			return id, true, nil
		}
	}
	return "", false, nil
}

func (f *fakeReplayCache) ListChunks(ctx context.Context, archiveID string) ([]ports.ReplayChunkRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	refs := []ports.ReplayChunkRef{}
	for index, part := range f.archives[archiveID].parts {
		refs = append(refs, ports.ReplayChunkRef{ID: int64(index), Dataset: part.Dataset, DriverNumber: part.DriverNumber})
	}
	return refs, nil
}

func (f *fakeReplayCache) ChunkRows(ctx context.Context, chunkID int64) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, archive := range f.archives {
		if archive.complete && int(chunkID) < len(archive.parts) {
			return archive.parts[chunkID].Rows, nil
		}
	}
	return nil, errors.New("no chunk")
}

func (f *fakeReplayCache) Create(ctx context.Context, sessionID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := "archive-" + strconv.Itoa(f.nextID)
	f.archives[id] = &fakeArchive{sessionID: sessionID}
	return id, nil
}

func (f *fakeReplayCache) SaveChunk(ctx context.Context, archiveID string, index int, part domain.ReplayPart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	archive, ok := f.archives[archiveID]
	if !ok {
		return errors.New("archive gone")
	}
	if index != len(archive.parts) {
		return errors.New("chunk index out of sequence")
	}
	archive.parts = append(archive.parts, part)
	return nil
}

// MarkComplete mirrors the repository: an archive is completed only when the session has no
// complete archive yet, and a complete archive is never deleted.
func (f *fakeReplayCache) MarkComplete(ctx context.Context, archiveID string, sessionID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	archive, ok := f.archives[archiveID]
	if !ok {
		return false, nil
	}
	for id, other := range f.archives {
		if id != archiveID && other.sessionID == sessionID && other.complete {
			return false, nil
		}
	}
	archive.complete = true
	return true, nil
}

func (f *fakeReplayCache) Delete(ctx context.Context, archiveID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.archives, archiveID)
	return nil
}

func (f *fakeReplayCache) Invalidate(ctx context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invalidated = append(f.invalidated, sessionID)
	for id, archive := range f.archives {
		if archive.sessionID == sessionID {
			delete(f.archives, id)
		}
	}
	return nil
}

func (f *fakeReplayCache) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.archives)
}

// fakeChampionshipClient only implements the subset RaceReplayUseCase
// actually calls (GetSession); every other method is unused in this test.
type fakeChampionshipClient struct {
	ports.ChampionshipClient
	session *ports.ChampionshipSessionRef
}

func (f *fakeChampionshipClient) GetSession(ctx context.Context, sessionID string) (*ports.ChampionshipSessionRef, error) {
	return f.session, nil
}

func t0(offsetSeconds int) time.Time {
	base := time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetSeconds) * time.Second)
}

// twoDriverRepository holds samples for drivers 63, 11 and 44 (44 has no sample of some
// datasets), plus one track-wide event of each kind.
func twoDriverRepository() *fakeRaceStreamRepository {
	lapDuration := 91.2
	return &fakeRaceStreamRepository{
		drivers: []int{63, 11, 44},
		telemetry: []domain.RaceReplayTelemetryFrame{
			{DriverNumber: 63, Speed: 300, Timestamp: t0(1)},
			{DriverNumber: 63, Speed: 310, Timestamp: t0(5)},
			{DriverNumber: 44, Speed: 290, Timestamp: t0(2)},
			{DriverNumber: 11, Speed: 280, Timestamp: t0(2)},
		},
		positions:   []domain.RacePosition{{DriverNumber: 63, Position: 1, Timestamp: t0(2)}, {DriverNumber: 44, Position: 2, Timestamp: t0(2)}},
		raceControl: []domain.RaceControlEvent{{Message: "green flag", Timestamp: t0(0)}},
		radio:       []domain.RaceRadioMessage{{DriverNumber: 63, Timestamp: t0(4)}},
		pitStops:    []ports.RaceReplayPitStopFrame{{Payload: domain.RacePitStop{DriverNumber: 63, LapNumber: 18}, Timestamp: t0(3)}},
		stints:      []ports.RaceReplayStintFrame{{Payload: domain.RaceStint{DriverNumber: 44, LapStart: 1}, Timestamp: t0(6)}},
		laps:        []ports.RaceReplayLapFrame{{Payload: domain.RaceReplayLapEvent{DriverNumber: 63, LapNumber: 1, LapDuration: &lapDuration}, Timestamp: t0(7)}},
	}
}

// partKey labels a part as "dataset/driver" (or "dataset" for track-wide) for order checks.
func partKey(part domain.ReplayPart) string {
	if part.DriverNumber == nil {
		return part.Dataset
	}
	return part.Dataset + "/" + strconv.Itoa(*part.DriverNumber)
}

// collect runs StreamReplay and returns the parts it emitted.
func collect(t *testing.T, uc *RaceReplayUseCase, query domain.ReplayQuery) []domain.ReplayPart {
	t.Helper()
	var parts []domain.ReplayPart
	err := uc.StreamReplay(context.Background(), "openf1:session:9998", query, func(part domain.ReplayPart) error {
		parts = append(parts, part)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamReplay: %v", err)
	}
	return parts
}

func keys(parts []domain.ReplayPart) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, partKey(part))
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestRaceReplayUseCase_SessionExists mirrors RaceLiveUseCase's/TelemetryUseCase's SessionExists
// coverage - the same "unknown sessionId -> 404" contract backs GET /race/replay.
func TestRaceReplayUseCase_SessionExists(t *testing.T) {
	t.Run("known session", func(t *testing.T) {
		uc := NewRaceReplayStreamUseCase(&fakeRaceStreamRepository{}, nil, &fakeChampionshipClient{session: &ports.ChampionshipSessionRef{ID: "s1"}}, 1, 8)
		exists, err := uc.SessionExists(context.Background(), "s1")
		if err != nil || !exists {
			t.Fatalf("expected exists=true, err=nil, got exists=%v err=%v", exists, err)
		}
	})

	t.Run("unknown session", func(t *testing.T) {
		uc := NewRaceReplayStreamUseCase(&fakeRaceStreamRepository{}, nil, &fakeChampionshipClient{session: nil}, 1, 8)
		exists, err := uc.SessionExists(context.Background(), "missing")
		if err != nil || exists {
			t.Fatalf("expected exists=false, err=nil, got exists=%v err=%v", exists, err)
		}
	})
}

// TestStreamReplay_PartsInDumpOrder proves the parts follow the dump order (datasets in
// ReplayDatasets order, drivers by string key "11" < "44" < "63"), that a driver with no sample
// in a dataset gets no part, that track-wide datasets are always emitted (`[]` when empty) and
// that each part holds that driver's samples, oldest first.
func TestStreamReplay_PartsInDumpOrder(t *testing.T) {
	uc := NewRaceReplayStreamUseCase(twoDriverRepository(), nil, &fakeChampionshipClient{}, 1, 8)

	parts := collect(t, uc, domain.ReplayQuery{})

	want := []string{
		"telemetry/11", "telemetry/44", "telemetry/63",
		"position/44", "position/63",
		"radio/63", "pitStop/63", "stint/44", "lap/63",
		"raceControl", "weather",
	}
	if got := keys(parts); !equalStrings(got, want) {
		t.Fatalf("parts = %v, want %v", got, want)
	}
	var telemetry63 []domain.RaceReplayTelemetrySample
	if err := json.Unmarshal(parts[2].Rows, &telemetry63); err != nil {
		t.Fatalf("telemetry/63 rows: %v", err)
	}
	if len(telemetry63) != 2 || telemetry63[0].Speed != 300 || telemetry63[1].Speed != 310 {
		t.Fatalf("telemetry/63 = %+v, want driver 63's two samples oldest first", telemetry63)
	}
	if string(parts[len(parts)-1].Rows) != "[]" {
		t.Fatalf("empty weather = %s, want []", parts[len(parts)-1].Rows)
	}
}

// TestStreamReplay_OneDriver proves driverNumber reads only that driver (the driver list is not
// queried) while track-wide datasets are still included.
func TestStreamReplay_OneDriver(t *testing.T) {
	repo := twoDriverRepository()
	uc := NewRaceReplayStreamUseCase(repo, nil, &fakeChampionshipClient{}, 1, 8)
	driver := 44

	parts := collect(t, uc, domain.ReplayQuery{DriverNumber: &driver})

	want := []string{"telemetry/44", "position/44", "stint/44", "raceControl", "weather"}
	if got := keys(parts); !equalStrings(got, want) {
		t.Fatalf("parts = %v, want %v", got, want)
	}
	if repo.driverCalls != 0 {
		t.Fatalf("driver list queried %d times for a one-driver replay", repo.driverCalls)
	}
}

// TestStreamReplay_WindowPassedToRepository proves the time window reaches every repository
// read, and that a window whose start is after its end is refused before any read.
func TestStreamReplay_WindowPassedToRepository(t *testing.T) {
	repo := twoDriverRepository()
	uc := NewRaceReplayStreamUseCase(repo, nil, &fakeChampionshipClient{}, 1, 8)
	from, to := t0(1), t0(4)

	collect(t, uc, domain.ReplayQuery{Window: domain.TimeWindow{From: &from, To: &to}})
	for _, window := range repo.windows {
		if window.From == nil || !window.From.Equal(from) || window.To == nil || !window.To.Equal(to) {
			t.Fatalf("repository read with window %+v, want [%s, %s]", window, from, to)
		}
	}

	repo.calls = 0
	err := uc.StreamReplay(context.Background(), "s", domain.ReplayQuery{Window: domain.TimeWindow{From: &to, To: &from}}, func(domain.ReplayPart) error { return nil })
	if !errors.Is(err, domain.ErrInvalidTimeWindow) || repo.calls != 0 {
		t.Fatalf("inverted window: err = %v, reads = %d; want ErrInvalidTimeWindow and no read", err, repo.calls)
	}
}

// TestStreamReplay_ConcurrencyCap proves a replay started while the cap is reached is refused
// with ErrReplayBusy before emitting anything, and that the slot is released afterwards.
func TestStreamReplay_ConcurrencyCap(t *testing.T) {
	uc := NewRaceReplayStreamUseCase(twoDriverRepository(), nil, &fakeChampionshipClient{}, 1, 8)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		first := true
		done <- uc.StreamReplay(context.Background(), "s", domain.ReplayQuery{}, func(domain.ReplayPart) error {
			if first {
				first = false
				close(entered)
				<-release
			}
			return nil
		})
	}()
	<-entered

	emitted := false
	err := uc.StreamReplay(context.Background(), "s", domain.ReplayQuery{}, func(domain.ReplayPart) error {
		emitted = true
		return nil
	})
	if !errors.Is(err, domain.ErrReplayBusy) || emitted {
		t.Fatalf("second replay: err = %v emitted=%v, want ErrReplayBusy before any part", err, emitted)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first replay: %v", err)
	}
	collect(t, uc, domain.ReplayQuery{})
}

// TestStreamReplay_CacheRecordsThenServes proves a full replay is recorded part by part and
// marked complete, and that the next replay (full, or one driver) is served from the cache
// without reading the sample tables, with the same parts.
func TestStreamReplay_CacheRecordsThenServes(t *testing.T) {
	repo := twoDriverRepository()
	cache := newFakeReplayCache()
	uc := NewRaceReplayStreamUseCase(repo, cache, &fakeChampionshipClient{}, 1, 8)

	first := collect(t, uc, domain.ReplayQuery{})
	if _, found, _ := cache.FindComplete(context.Background(), "openf1:session:9998"); !found {
		t.Fatal("full replay was not recorded as a complete archive")
	}

	repo.calls = 0
	second := collect(t, uc, domain.ReplayQuery{})
	if repo.calls != 0 {
		t.Fatalf("cached replay read the sample tables %d times", repo.calls)
	}
	if !equalStrings(keys(first), keys(second)) {
		t.Fatalf("cached parts %v differ from built parts %v", keys(second), keys(first))
	}
	for i := range first {
		if string(first[i].Rows) != string(second[i].Rows) {
			t.Fatalf("part %s rows differ from the cache", partKey(first[i]))
		}
	}

	driver := 63
	oneDriver := collect(t, uc, domain.ReplayQuery{DriverNumber: &driver})
	want := []string{"telemetry/63", "position/63", "radio/63", "pitStop/63", "lap/63", "raceControl", "weather"}
	if got := keys(oneDriver); !equalStrings(got, want) || repo.calls != 0 {
		t.Fatalf("cached one-driver parts = %v (reads %d), want %v from the cache", got, repo.calls, want)
	}
}

// TestStreamReplay_PartialQueriesAreNotRecorded proves a one-driver or windowed replay never
// creates an archive (it would be an incomplete dump), and that a windowed replay skips the
// cache even when one exists.
func TestStreamReplay_PartialQueriesAreNotRecorded(t *testing.T) {
	repo := twoDriverRepository()
	cache := newFakeReplayCache()
	uc := NewRaceReplayStreamUseCase(repo, cache, &fakeChampionshipClient{}, 1, 8)
	driver := 63
	from := t0(1)

	collect(t, uc, domain.ReplayQuery{DriverNumber: &driver})
	collect(t, uc, domain.ReplayQuery{Window: domain.TimeWindow{From: &from}})
	if cache.count() != 0 {
		t.Fatalf("%d archives created by partial replays", cache.count())
	}

	collect(t, uc, domain.ReplayQuery{})
	repo.calls = 0
	collect(t, uc, domain.ReplayQuery{Window: domain.TimeWindow{From: &from}})
	if repo.calls == 0 {
		t.Fatal("windowed replay was served from the full-dump cache")
	}
}

// TestStreamReplay_ClientGoneDropsArchive proves an emit failure (client disconnected) stops
// the replay with that error and deletes the unfinished archive.
func TestStreamReplay_ClientGoneDropsArchive(t *testing.T) {
	cache := newFakeReplayCache()
	uc := NewRaceReplayStreamUseCase(twoDriverRepository(), cache, &fakeChampionshipClient{}, 1, 8)
	gone := errors.New("broken pipe")
	sent := 0

	err := uc.StreamReplay(context.Background(), "s", domain.ReplayQuery{}, func(domain.ReplayPart) error {
		sent++
		if sent == 3 {
			return gone
		}
		return nil
	})
	if !errors.Is(err, gone) {
		t.Fatalf("err = %v, want the emit error", err)
	}
	if cache.count() != 0 {
		t.Fatal("unfinished archive kept after the client left")
	}
}

// TestStreamReplay_CacheWriteFailureKeepsStreaming proves a failing cache write only stops the
// recording: the client still gets every part and no archive is left behind.
func TestStreamReplay_CacheWriteFailureKeepsStreaming(t *testing.T) {
	cache := newFakeReplayCache()
	cache.saveErr = errors.New("disk full")
	uc := NewRaceReplayStreamUseCase(twoDriverRepository(), cache, &fakeChampionshipClient{}, 1, 8)

	parts := collect(t, uc, domain.ReplayQuery{})
	if len(parts) != 11 {
		t.Fatalf("got %d parts, want the full dump", len(parts))
	}
	if cache.count() != 0 {
		t.Fatal("archive kept after a failed chunk write")
	}
}

// TestStreamReplay_CacheLookupFailureFallsBack proves an unreachable cache does not fail the
// replay: it is built from the sample tables.
func TestStreamReplay_CacheLookupFailureFallsBack(t *testing.T) {
	cache := newFakeReplayCache()
	cache.findErr = errors.New("db timeout")
	uc := NewRaceReplayStreamUseCase(twoDriverRepository(), cache, &fakeChampionshipClient{}, 1, 8)

	if parts := collect(t, uc, domain.ReplayQuery{}); len(parts) != 11 {
		t.Fatalf("got %d parts, want the full dump", len(parts))
	}
}

// TestStreamReplay_RepositoryErrorDropsArchive proves a sample read failure is returned and the
// archive being recorded is deleted.
func TestStreamReplay_RepositoryErrorDropsArchive(t *testing.T) {
	repo := twoDriverRepository()
	repo.err = errors.New("db down")
	cache := newFakeReplayCache()
	uc := NewRaceReplayStreamUseCase(repo, cache, &fakeChampionshipClient{}, 1, 8)

	err := uc.StreamReplay(context.Background(), "s", domain.ReplayQuery{}, func(domain.ReplayPart) error { return nil })
	if err == nil {
		t.Fatal("StreamReplay = nil, want the repository error")
	}
	if cache.count() != 0 {
		t.Fatal("archive kept after a repository failure")
	}
}

// TestStreamReplay_EmptySession proves a session without samples yields only the two
// track-wide datasets, as empty arrays.
func TestStreamReplay_EmptySession(t *testing.T) {
	uc := NewRaceReplayStreamUseCase(&fakeRaceStreamRepository{}, nil, &fakeChampionshipClient{}, 1, 8)

	parts := collect(t, uc, domain.ReplayQuery{})
	if got := keys(parts); !equalStrings(got, []string{"raceControl", "weather"}) {
		t.Fatalf("parts = %v", got)
	}
	for _, part := range parts {
		if string(part.Rows) != "[]" {
			t.Fatalf("%s = %s, want []", part.Dataset, part.Rows)
		}
	}
}

// TestStreamReplay_ConcurrentRecordingsKeepTheFirstArchive proves two full replays recorded at
// the same time leave exactly one complete archive: the recording that finishes second drops its
// own copy instead of deleting the one already served.
func TestStreamReplay_ConcurrentRecordingsKeepTheFirstArchive(t *testing.T) {
	cache := newFakeReplayCache()
	uc := NewRaceReplayStreamUseCase(twoDriverRepository(), cache, &fakeChampionshipClient{}, 2, 8)
	firstPart := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		first := true
		done <- uc.StreamReplay(context.Background(), "openf1:session:9998", domain.ReplayQuery{}, func(domain.ReplayPart) error {
			if first {
				first = false
				close(firstPart)
				<-release
			}
			return nil
		})
	}()
	<-firstPart

	collect(t, uc, domain.ReplayQuery{}) // second recording, completes first
	firstID, found, _ := cache.FindComplete(context.Background(), "openf1:session:9998")
	if !found {
		t.Fatal("no complete archive after the second recording")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first recording: %v", err)
	}

	if cache.count() != 1 {
		t.Fatalf("%d archives left, want exactly one", cache.count())
	}
	if id, _, _ := cache.FindComplete(context.Background(), "openf1:session:9998"); id != firstID {
		t.Fatalf("complete archive %s replaced by %s while it could be streamed", firstID, id)
	}
}

// TestStreamReplay_CachedReplayTakesNoSlot proves a replay served from the cache does not hold a
// build slot: with the only slot taken by a running build, cached replays are still served, and
// a slow cached reader does not make a build answer ErrReplayBusy.
func TestStreamReplay_CachedReplayTakesNoSlot(t *testing.T) {
	cache := newFakeReplayCache()
	uc := NewRaceReplayStreamUseCase(twoDriverRepository(), cache, &fakeChampionshipClient{}, 1, 8)
	collect(t, uc, domain.ReplayQuery{}) // records the archive of 9998

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		first := true
		done <- uc.StreamReplay(context.Background(), "openf1:session:9998", domain.ReplayQuery{}, func(domain.ReplayPart) error {
			if first {
				first = false
				close(entered)
				<-release
			}
			return nil
		})
	}()
	<-entered // a slow reader of the cached dump

	from := t0(1)
	if parts := collect(t, uc, domain.ReplayQuery{Window: domain.TimeWindow{From: &from}}); len(parts) == 0 {
		t.Fatal("windowed build refused while only a cached reader was running")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("cached reader: %v", err)
	}
}

// TestStreamReplay_CacheReadCap proves cached replays have their own cap: once every cache slot
// is held by a reader, the next cached replay is refused with ErrReplayBusy before any part.
func TestStreamReplay_CacheReadCap(t *testing.T) {
	cache := newFakeReplayCache()
	uc := NewRaceReplayStreamUseCase(twoDriverRepository(), cache, &fakeChampionshipClient{}, 1, 1)
	collect(t, uc, domain.ReplayQuery{})

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		first := true
		done <- uc.StreamReplay(context.Background(), "openf1:session:9998", domain.ReplayQuery{}, func(domain.ReplayPart) error {
			if first {
				first = false
				close(entered)
				<-release
			}
			return nil
		})
	}()
	<-entered

	emitted := false
	err := uc.StreamReplay(context.Background(), "openf1:session:9998", domain.ReplayQuery{}, func(domain.ReplayPart) error {
		emitted = true
		return nil
	})
	if !errors.Is(err, domain.ErrReplayBusy) || emitted {
		t.Fatalf("second cached reader: err=%v emitted=%v, want ErrReplayBusy before any part", err, emitted)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first reader: %v", err)
	}
}
