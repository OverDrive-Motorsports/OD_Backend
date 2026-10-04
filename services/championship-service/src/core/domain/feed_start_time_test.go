/**
##
## OverDrive 2026
## All Technical rights reserved
##
## feed_start_time_test.go - Unit tests for the feed start-time rules (window check, UTC normalisation).
##
*/

package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestValidateFeedStartTimes_Window proves the accepted range is the session window widened by
// FeedStartTolerance on both sides (bounds inclusive), that the error names the failing index
// and field, and that feeds without a start time are ignored.
func TestValidateFeedStartTimes_Window(t *testing.T) {
	start := time.Date(2026, 7, 6, 13, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 6, 15, 0, 0, 0, time.UTC)
	window := SessionWindow{Start: start, End: &end}
	at := func(value time.Time) []Feed {
		return []Feed{{Provider: FeedProviderHLS, URL: "https://a.b/x.m3u8"}, {Provider: FeedProviderYouTube, URL: "https://youtu.be/a", StartedAtUTC: &value}}
	}

	for name, value := range map[string]time.Time{
		"lower bound":   start.Add(-FeedStartTolerance),
		"inside":        start.Add(-3 * time.Minute),
		"upper bound":   end.Add(FeedStartTolerance),
		"non-UTC input": start.In(time.FixedZone("CEST", 2*3600)),
	} {
		if err := ValidateFeedStartTimes(at(value), window); err != nil {
			t.Fatalf("%s: unexpected error %v", name, err)
		}
	}

	for name, value := range map[string]time.Time{
		"just before":  start.Add(-FeedStartTolerance - time.Second),
		"just after":   end.Add(FeedStartTolerance + time.Second),
		"wrong year":   start.AddDate(-1, 0, 0),
		"zero instant": {},
	} {
		err := ValidateFeedStartTimes(at(value), window)
		if !errors.Is(err, ErrInvalidFeed) || !strings.Contains(err.Error(), "feeds[1].startedAtUtc") {
			t.Fatalf("%s: expected ErrInvalidFeed naming feeds[1].startedAtUtc, got %v", name, err)
		}
	}
}

// TestValidateFeedStartTimes_OpenSessionAndUnknownBounds proves a session with no end uses
// FeedOpenSessionHorizon past its start as the upper bound, and a zero window start disables
// the check entirely (bounds unknown).
func TestValidateFeedStartTimes_OpenSessionAndUnknownBounds(t *testing.T) {
	start := time.Date(2026, 7, 6, 13, 0, 0, 0, time.UTC)
	inHorizon := start.Add(FeedOpenSessionHorizon)
	pastHorizon := inHorizon.Add(time.Second)
	open := SessionWindow{Start: start}

	if err := ValidateFeedStartTimes([]Feed{{StartedAtUTC: &inHorizon}}, open); err != nil {
		t.Fatalf("expected the horizon bound to be accepted, got %v", err)
	}
	if err := ValidateFeedStartTimes([]Feed{{StartedAtUTC: &pastHorizon}}, open); !errors.Is(err, ErrInvalidFeed) {
		t.Fatalf("expected ErrInvalidFeed past the horizon, got %v", err)
	}
	if err := ValidateFeedStartTimes([]Feed{{StartedAtUTC: &pastHorizon}}, SessionWindow{}); err != nil {
		t.Fatalf("expected no check with unknown bounds, got %v", err)
	}
}

// TestHasFeedStartTimes proves it is true only when at least one feed carries a start time.
func TestHasFeedStartTimes(t *testing.T) {
	now := time.Now()
	if HasFeedStartTimes(nil) || HasFeedStartTimes([]Feed{{Provider: FeedProviderHLS}}) {
		t.Fatal("expected false without any start time")
	}
	if !HasFeedStartTimes([]Feed{{Provider: FeedProviderHLS}, {StartedAtUTC: &now}}) {
		t.Fatal("expected true when one feed has a start time")
	}
}

// TestNormalizeFeeds proves start times are converted to UTC (same instant), feeds without one
// are untouched, the input slice is not mutated, and nil becomes an empty non-nil list.
func TestNormalizeFeeds(t *testing.T) {
	zone := time.FixedZone("CEST", 2*3600)
	local := time.Date(2026, 7, 6, 16, 0, 0, 0, zone)
	input := []Feed{{Provider: FeedProviderYouTube, URL: "https://youtu.be/a", StartedAtUTC: &local}, {Provider: FeedProviderHLS, URL: "https://a.b/x.m3u8"}}

	got := NormalizeFeeds(input)
	if got[0].StartedAtUTC.Location() != time.UTC || !got[0].StartedAtUTC.Equal(local) {
		t.Fatalf("expected the same instant in UTC, got %v", got[0].StartedAtUTC)
	}
	if got[1] != input[1] {
		t.Fatalf("expected a feed without start time unchanged, got %+v", got[1])
	}
	if input[0].StartedAtUTC.Location() != zone {
		t.Fatal("input must not be mutated")
	}
	if empty := NormalizeFeeds(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("expected an empty non-nil list, got %#v", empty)
	}
}

// TestFeed_StartedAtUtcJSON proves the wire format: RFC 3339 under `startedAtUtc`, omitted when
// absent, and a non-RFC 3339 value fails to decode (the controller turns that into a 400).
func TestFeed_StartedAtUtcJSON(t *testing.T) {
	at := time.Date(2026, 7, 6, 14, 2, 51, 0, time.UTC)
	raw, _ := json.Marshal(Feed{Provider: FeedProviderHLS, URL: "https://a.b/x.m3u8", StartedAtUTC: &at})
	if !strings.Contains(string(raw), `"startedAtUtc":"2026-07-06T14:02:51Z"`) {
		t.Fatalf("unexpected encoding: %s", raw)
	}
	raw, _ = json.Marshal(Feed{Provider: FeedProviderHLS, URL: "https://a.b/x.m3u8"})
	if strings.Contains(string(raw), "startedAtUtc") {
		t.Fatalf("expected startedAtUtc omitted when absent: %s", raw)
	}
	var decoded Feed
	if err := json.Unmarshal([]byte(`{"provider":"hls","startedAtUtc":"06/07/2026 14:02"}`), &decoded); err == nil {
		t.Fatal("expected a non-RFC 3339 start time to be rejected")
	}
}
