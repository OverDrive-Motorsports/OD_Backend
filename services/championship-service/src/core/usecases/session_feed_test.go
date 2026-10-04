/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_feed_test.go - Unit tests for SessionFeedUseCase with a fake SessionFeedRepository.
##
*/

package usecases

import (
	"context"
	"errors"
	"testing"

	"overdrive/services/championship-service/src/core/domain"
)

// fakeSessionFeedRepository records the last ReplaceSessionFeeds call and returns canned results.
type fakeSessionFeedRepository struct {
	result        *domain.SessionBroadcast
	err           error
	replaceCalled bool
	gotSessionID  string
	gotFeeds      []domain.Feed
}

func (f *fakeSessionFeedRepository) GetSessionFeeds(_ context.Context, sessionID string) (*domain.SessionBroadcast, error) {
	f.gotSessionID = sessionID
	return f.result, f.err
}

func (f *fakeSessionFeedRepository) ReplaceSessionFeeds(_ context.Context, sessionID string, feeds []domain.Feed) (*domain.SessionBroadcast, error) {
	f.replaceCalled = true
	f.gotSessionID = sessionID
	f.gotFeeds = feeds
	return f.result, f.err
}

// TestSessionFeedUseCase_GetSessionFeeds proves the read path is a pass-through (result, nil
// for unknown session, and repository error) forwarding the session ID unchanged.
func TestSessionFeedUseCase_GetSessionFeeds(t *testing.T) {
	want := &domain.SessionBroadcast{SessionID: "s1", Feeds: []domain.Feed{}}
	repo := &fakeSessionFeedRepository{result: want}
	got, err := NewSessionFeedUseCase(repo).GetSessionFeeds(context.Background(), "s1")
	if err != nil || got != want || repo.gotSessionID != "s1" {
		t.Fatalf("unexpected result: %+v err=%v sessionID=%q", got, err, repo.gotSessionID)
	}

	repo = &fakeSessionFeedRepository{}
	got, err = NewSessionFeedUseCase(repo).GetSessionFeeds(context.Background(), "missing")
	if err != nil || got != nil {
		t.Fatalf("expected (nil, nil) for an unknown session, got %+v err=%v", got, err)
	}

	boom := errors.New("db down")
	repo = &fakeSessionFeedRepository{err: boom}
	if _, err := NewSessionFeedUseCase(repo).GetSessionFeeds(context.Background(), "s1"); !errors.Is(err, boom) {
		t.Fatalf("expected repository error to propagate, got %v", err)
	}
}

// TestSessionFeedUseCase_ReplaceSessionFeeds_NormalCase proves a valid list is forwarded to the
// repository unchanged and the stored payload is returned as-is.
func TestSessionFeedUseCase_ReplaceSessionFeeds_NormalCase(t *testing.T) {
	feeds := []domain.Feed{{Provider: domain.FeedProviderHLS, URL: "https://cdn.example.com/a.m3u8"}}
	want := &domain.SessionBroadcast{SessionID: "s1", Feeds: feeds}
	repo := &fakeSessionFeedRepository{result: want}

	got, err := NewSessionFeedUseCase(repo).ReplaceSessionFeeds(context.Background(), "s1", feeds)
	if err != nil || got != want {
		t.Fatalf("unexpected result: %+v err=%v", got, err)
	}
	if !repo.replaceCalled || repo.gotSessionID != "s1" || len(repo.gotFeeds) != 1 || repo.gotFeeds[0] != feeds[0] {
		t.Fatalf("expected feeds forwarded to repository, got called=%v id=%q feeds=%+v", repo.replaceCalled, repo.gotSessionID, repo.gotFeeds)
	}
}

// TestSessionFeedUseCase_ReplaceSessionFeeds_InvalidListNeverReachesRepository proves a list
// rejected by domain.ValidateFeeds surfaces ErrInvalidFeed and the repository is never called.
func TestSessionFeedUseCase_ReplaceSessionFeeds_InvalidListNeverReachesRepository(t *testing.T) {
	repo := &fakeSessionFeedRepository{result: &domain.SessionBroadcast{SessionID: "s1"}}
	_, err := NewSessionFeedUseCase(repo).ReplaceSessionFeeds(context.Background(), "s1", []domain.Feed{
		{Provider: domain.FeedProviderF1TV, ContentID: "1", ChannelID: "2", URL: "https://f1tv.example/manifest.mpd"},
	})
	if !errors.Is(err, domain.ErrInvalidFeed) {
		t.Fatalf("expected ErrInvalidFeed, got %v", err)
	}
	if repo.replaceCalled {
		t.Fatal("repository must not be called for an invalid list")
	}
}

// TestSessionFeedUseCase_ReplaceSessionFeeds_NilBecomesEmpty proves a nil list is normalised to
// an empty (non-nil) slice before persistence, so the column always holds a JSON array.
func TestSessionFeedUseCase_ReplaceSessionFeeds_NilBecomesEmpty(t *testing.T) {
	repo := &fakeSessionFeedRepository{result: &domain.SessionBroadcast{SessionID: "s1", Feeds: []domain.Feed{}}}
	if _, err := NewSessionFeedUseCase(repo).ReplaceSessionFeeds(context.Background(), "s1", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotFeeds == nil || len(repo.gotFeeds) != 0 {
		t.Fatalf("expected an empty non-nil slice, got %#v", repo.gotFeeds)
	}
}

// TestSessionFeedUseCase_ReplaceSessionFeeds_UnknownSessionAndRepositoryError proves the
// repository's (nil, nil) for an unknown session and its errors are returned unchanged.
func TestSessionFeedUseCase_ReplaceSessionFeeds_UnknownSessionAndRepositoryError(t *testing.T) {
	repo := &fakeSessionFeedRepository{}
	got, err := NewSessionFeedUseCase(repo).ReplaceSessionFeeds(context.Background(), "missing", []domain.Feed{})
	if err != nil || got != nil {
		t.Fatalf("expected (nil, nil) for an unknown session, got %+v err=%v", got, err)
	}

	boom := errors.New("db down")
	repo = &fakeSessionFeedRepository{err: boom}
	if _, err := NewSessionFeedUseCase(repo).ReplaceSessionFeeds(context.Background(), "s1", []domain.Feed{}); !errors.Is(err, boom) {
		t.Fatalf("expected repository error to propagate, got %v", err)
	}
}
