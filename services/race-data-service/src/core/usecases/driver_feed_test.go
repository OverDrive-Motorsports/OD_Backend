/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed_test.go - Unit tests for DriverFeedUseCase with fake repository and championship client.
##
*/

package usecases

import (
	"context"
	"errors"
	"testing"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// fakeChampionshipClientForDriverFeed only implements ListSessionDrivers; every other method
// of ports.ChampionshipClient panics via the embedded nil interface if reached.
type fakeChampionshipClientForDriverFeed struct {
	ports.ChampionshipClient
	drivers []ports.ChampionshipDriverRef
	err     error
	gotID   string
}

func (f *fakeChampionshipClientForDriverFeed) ListSessionDrivers(_ context.Context, sessionID string) ([]ports.ChampionshipDriverRef, error) {
	f.gotID = sessionID
	return f.drivers, f.err
}

// fakeDriverFeedRepository records the last replace call and returns canned results.
type fakeDriverFeedRepository struct {
	feeds           []domain.Feed
	err             error
	replaceCalled   bool
	gotSessionID    string
	gotDriverNumber int
	gotFeeds        []domain.Feed
}

func (f *fakeDriverFeedRepository) GetDriverFeeds(_ context.Context, sessionID string, driverNumber int) ([]domain.Feed, error) {
	f.gotSessionID = sessionID
	f.gotDriverNumber = driverNumber
	return f.feeds, f.err
}

func (f *fakeDriverFeedRepository) ReplaceDriverFeeds(_ context.Context, sessionID string, driverNumber int, feeds []domain.Feed) ([]domain.Feed, error) {
	f.replaceCalled = true
	f.gotSessionID = sessionID
	f.gotDriverNumber = driverNumber
	f.gotFeeds = feeds
	if f.err != nil {
		return nil, f.err
	}
	return feeds, nil
}

func knownDrivers() []ports.ChampionshipDriverRef {
	return []ports.ChampionshipDriverRef{{DriverNumber: 1, DriverName: "Max"}, {DriverNumber: 44, DriverName: "Lewis"}}
}

// TestDriverFeedUseCase_GetDriverFeeds_NormalCase proves a known session+driver returns the
// repository's list wrapped in a DriverBroadcast, and a driver with no row yields `[]` not nil.
func TestDriverFeedUseCase_GetDriverFeeds_NormalCase(t *testing.T) {
	feeds := []domain.Feed{{Provider: domain.FeedProviderF1TV, ContentID: "1", ChannelID: "2"}}
	repo := &fakeDriverFeedRepository{feeds: feeds}
	client := &fakeChampionshipClientForDriverFeed{drivers: knownDrivers()}
	got, err := NewDriverFeedUseCase(repo, client).GetDriverFeeds(context.Background(), "s1", 44)
	if err != nil || got == nil || got.SessionID != "s1" || got.DriverNumber != 44 || len(got.Feeds) != 1 || got.Feeds[0] != feeds[0] {
		t.Fatalf("unexpected result: %+v err=%v", got, err)
	}
	if client.gotID != "s1" || repo.gotSessionID != "s1" || repo.gotDriverNumber != 44 {
		t.Fatalf("expected identifiers forwarded, got client=%q repo=%q/%d", client.gotID, repo.gotSessionID, repo.gotDriverNumber)
	}

	got, err = NewDriverFeedUseCase(&fakeDriverFeedRepository{feeds: nil}, client).GetDriverFeeds(context.Background(), "s1", 1)
	if err != nil || got.Feeds == nil || len(got.Feeds) != 0 {
		t.Fatalf("expected empty non-nil feeds, got %#v err=%v", got, err)
	}
}

// TestDriverFeedUseCase_UnknownSessionAndDriver proves a nil driver list (championship-service
// 404) maps to ErrSessionNotFound and an absent driver number to ErrDriverNotFound, on both
// the read and the write path, without touching the repository.
func TestDriverFeedUseCase_UnknownSessionAndDriver(t *testing.T) {
	repo := &fakeDriverFeedRepository{}
	unknownSession := NewDriverFeedUseCase(repo, &fakeChampionshipClientForDriverFeed{drivers: nil})
	if _, err := unknownSession.GetDriverFeeds(context.Background(), "missing", 1); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("GET: expected ErrSessionNotFound, got %v", err)
	}
	if _, err := unknownSession.ReplaceDriverFeeds(context.Background(), "missing", 1, []domain.Feed{}); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("PUT: expected ErrSessionNotFound, got %v", err)
	}

	unknownDriver := NewDriverFeedUseCase(repo, &fakeChampionshipClientForDriverFeed{drivers: knownDrivers()})
	if _, err := unknownDriver.GetDriverFeeds(context.Background(), "s1", 99); !errors.Is(err, domain.ErrDriverNotFound) {
		t.Fatalf("GET: expected ErrDriverNotFound, got %v", err)
	}
	if _, err := unknownDriver.ReplaceDriverFeeds(context.Background(), "s1", 99, []domain.Feed{}); !errors.Is(err, domain.ErrDriverNotFound) {
		t.Fatalf("PUT: expected ErrDriverNotFound, got %v", err)
	}
	if repo.replaceCalled {
		t.Fatal("repository must not be written for an unknown session/driver")
	}
}

// TestDriverFeedUseCase_ChampionshipFailure proves a failing driver lookup is wrapped in
// ErrChampionshipUnavailable (still unwrapping to the original error) and aborts the write.
func TestDriverFeedUseCase_ChampionshipFailure(t *testing.T) {
	boom := errors.New("dial tcp: connection refused")
	repo := &fakeDriverFeedRepository{}
	uc := NewDriverFeedUseCase(repo, &fakeChampionshipClientForDriverFeed{err: boom})
	_, err := uc.ReplaceDriverFeeds(context.Background(), "s1", 1, []domain.Feed{})
	if !errors.Is(err, domain.ErrChampionshipUnavailable) || !errors.Is(err, boom) {
		t.Fatalf("expected ErrChampionshipUnavailable wrapping the cause, got %v", err)
	}
	if repo.replaceCalled {
		t.Fatal("repository must not be written when the driver lookup fails")
	}
	if _, err := uc.GetDriverFeeds(context.Background(), "s1", 1); !errors.Is(err, domain.ErrChampionshipUnavailable) {
		t.Fatalf("GET: expected ErrChampionshipUnavailable, got %v", err)
	}
}

// TestDriverFeedUseCase_ReplaceDriverFeeds_NormalCase proves a valid list is forwarded to the
// repository with the session/driver identifiers and returned in the payload.
func TestDriverFeedUseCase_ReplaceDriverFeeds_NormalCase(t *testing.T) {
	feeds := []domain.Feed{{Provider: domain.FeedProviderYouTube, URL: "https://youtu.be/abc", Label: "Onboard"}}
	repo := &fakeDriverFeedRepository{}
	got, err := NewDriverFeedUseCase(repo, &fakeChampionshipClientForDriverFeed{drivers: knownDrivers()}).ReplaceDriverFeeds(context.Background(), "s1", 44, feeds)
	if err != nil || got == nil || got.SessionID != "s1" || got.DriverNumber != 44 || len(got.Feeds) != 1 || got.Feeds[0] != feeds[0] {
		t.Fatalf("unexpected result: %+v err=%v", got, err)
	}
	if !repo.replaceCalled || repo.gotSessionID != "s1" || repo.gotDriverNumber != 44 || len(repo.gotFeeds) != 1 {
		t.Fatalf("unexpected repository call: %+v", repo)
	}
}

// TestDriverFeedUseCase_ReplaceDriverFeeds_InvalidListNeverReachesDownstream proves validation
// runs first: an invalid list returns ErrInvalidFeed without calling championship-service or
// the repository.
func TestDriverFeedUseCase_ReplaceDriverFeeds_InvalidListNeverReachesDownstream(t *testing.T) {
	repo := &fakeDriverFeedRepository{}
	client := &fakeChampionshipClientForDriverFeed{drivers: knownDrivers()}
	_, err := NewDriverFeedUseCase(repo, client).ReplaceDriverFeeds(context.Background(), "s1", 1, []domain.Feed{{Provider: "twitch", URL: "https://twitch.tv/x"}})
	if !errors.Is(err, domain.ErrInvalidFeed) {
		t.Fatalf("expected ErrInvalidFeed, got %v", err)
	}
	if repo.replaceCalled || client.gotID != "" {
		t.Fatal("neither the repository nor championship-service must be called for an invalid list")
	}
}

// TestDriverFeedUseCase_ReplaceDriverFeeds_NilBecomesEmptyAndErrorsPropagate proves a nil list
// is stored as an empty slice and a repository failure is surfaced unchanged.
func TestDriverFeedUseCase_ReplaceDriverFeeds_NilBecomesEmptyAndErrorsPropagate(t *testing.T) {
	repo := &fakeDriverFeedRepository{}
	client := &fakeChampionshipClientForDriverFeed{drivers: knownDrivers()}
	got, err := NewDriverFeedUseCase(repo, client).ReplaceDriverFeeds(context.Background(), "s1", 1, nil)
	if err != nil || repo.gotFeeds == nil || len(repo.gotFeeds) != 0 || got.Feeds == nil {
		t.Fatalf("expected empty non-nil feeds, repo=%#v got=%+v err=%v", repo.gotFeeds, got, err)
	}

	boom := errors.New("db down")
	if _, err := NewDriverFeedUseCase(&fakeDriverFeedRepository{err: boom}, client).ReplaceDriverFeeds(context.Background(), "s1", 1, []domain.Feed{}); !errors.Is(err, boom) {
		t.Fatalf("expected repository error to propagate, got %v", err)
	}
}
