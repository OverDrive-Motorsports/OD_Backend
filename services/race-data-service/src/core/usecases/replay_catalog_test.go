/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog_test.go - Unit tests for the replay catalog use case.
##
*/

package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// fakeChampionshipSessionCatalog returns canned championship sessions and records the code.
type fakeChampionshipSessionCatalog struct {
	sessions []ports.ChampionshipSessionRow
	err      error
	gotCode  string
}

func (f *fakeChampionshipSessionCatalog) ListChampionshipSessions(_ context.Context, code string) ([]ports.ChampionshipSessionRow, error) {
	f.gotCode = code
	return f.sessions, f.err
}

// fakeReplayCatalogRepository returns a canned playable set and records the candidate IDs.
type fakeReplayCatalogRepository struct {
	playable map[string]struct{}
	err      error
	called   bool
	gotIDs   []string
}

func (f *fakeReplayCatalogRepository) SessionsWithReplay(_ context.Context, ids []string) (map[string]struct{}, error) {
	f.called = true
	f.gotIDs = ids
	return f.playable, f.err
}

func championshipSessions() []ports.ChampionshipSessionRow {
	return []ports.ChampionshipSessionRow{
		{SessionID: "bahrain-race", Name: "Race", Type: "race", StartTime: time.Date(2024, 3, 2, 15, 0, 0, 0, time.UTC), EventName: "Bahrain Grand Prix", SeasonYear: 2024, Circuit: "Bahrain International Circuit"},
		{SessionID: "bahrain-fp1", Name: "Practice 1", Type: "practice", StartTime: time.Date(2024, 2, 29, 11, 30, 0, 0, time.UTC), EventName: "Bahrain Grand Prix", SeasonYear: 2024, Circuit: "Bahrain International Circuit"},
		{SessionID: "jeddah-race", Name: "Race", Type: "race", StartTime: time.Date(2024, 3, 9, 17, 0, 0, 0, time.UTC), EventName: "Saudi Arabian Grand Prix", SeasonYear: 2024, Circuit: "Jeddah Corniche Circuit"},
	}
}

// TestReplayCatalogUseCase_ListReplays_NormalCase proves every championship session ID is
// checked, only playable ones are returned, newest first, with the event context copied through.
func TestReplayCatalogUseCase_ListReplays_NormalCase(t *testing.T) {
	catalog := &fakeChampionshipSessionCatalog{sessions: championshipSessions()}
	repo := &fakeReplayCatalogRepository{playable: map[string]struct{}{"bahrain-race": {}, "jeddah-race": {}}}

	got, err := NewReplayCatalogUseCase(catalog, repo).ListReplays(context.Background(), "f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if catalog.gotCode != "f1" || len(repo.gotIDs) != 3 {
		t.Fatalf("expected code forwarded and 3 candidates checked, got code=%q ids=%v", catalog.gotCode, repo.gotIDs)
	}
	if len(got) != 2 || got[0].SessionID != "jeddah-race" || got[1].SessionID != "bahrain-race" {
		t.Fatalf("expected the 2 playable sessions newest first, got %+v", got)
	}
	want := domain.ReplayCatalogEntry{SessionID: "bahrain-race", Name: "Race", Type: "race", StartTime: championshipSessions()[0].StartTime, SeasonYear: 2024, EventName: "Bahrain Grand Prix", Circuit: "Bahrain International Circuit"}
	if got[1] != want {
		t.Fatalf("unexpected entry: %+v", got[1])
	}
}

// TestReplayCatalogUseCase_ListReplays_Empty proves `[]` (non-nil) both when no session is
// playable and when the championship has no session at all — the latter without a DB query.
func TestReplayCatalogUseCase_ListReplays_Empty(t *testing.T) {
	repo := &fakeReplayCatalogRepository{playable: map[string]struct{}{}}
	got, err := NewReplayCatalogUseCase(&fakeChampionshipSessionCatalog{sessions: championshipSessions()}, repo).ListReplays(context.Background(), "f1")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("expected an empty non-nil list, got %#v err=%v", got, err)
	}

	repo = &fakeReplayCatalogRepository{}
	got, err = NewReplayCatalogUseCase(&fakeChampionshipSessionCatalog{sessions: []ports.ChampionshipSessionRow{}}, repo).ListReplays(context.Background(), "f1")
	if err != nil || got == nil || len(got) != 0 || repo.called {
		t.Fatalf("expected [] without querying the store, got %#v err=%v called=%v", got, err, repo.called)
	}
}

// TestReplayCatalogUseCase_ListReplays_Errors proves an unknown championship is (nil, nil), a
// championship-service failure is wrapped in ErrChampionshipUnavailable (502), and a store
// failure propagates unwrapped (500).
func TestReplayCatalogUseCase_ListReplays_Errors(t *testing.T) {
	repo := &fakeReplayCatalogRepository{}
	got, err := NewReplayCatalogUseCase(&fakeChampionshipSessionCatalog{}, repo).ListReplays(context.Background(), "nope")
	if err != nil || got != nil || repo.called {
		t.Fatalf("expected (nil, nil) without querying the store, got %+v err=%v", got, err)
	}

	upstream := errors.New("timeout")
	_, err = NewReplayCatalogUseCase(&fakeChampionshipSessionCatalog{err: upstream}, repo).ListReplays(context.Background(), "f1")
	if !errors.Is(err, domain.ErrChampionshipUnavailable) || !errors.Is(err, upstream) {
		t.Fatalf("expected ErrChampionshipUnavailable wrapping the cause, got %v", err)
	}

	store := errors.New("db down")
	_, err = NewReplayCatalogUseCase(&fakeChampionshipSessionCatalog{sessions: championshipSessions()}, &fakeReplayCatalogRepository{err: store}).ListReplays(context.Background(), "f1")
	if !errors.Is(err, store) || errors.Is(err, domain.ErrChampionshipUnavailable) {
		t.Fatalf("expected the store error unwrapped, got %v", err)
	}
}
