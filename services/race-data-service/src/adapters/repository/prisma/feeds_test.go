/**
##
## OverDrive 2026
## All Technical rights reserved
##
## feeds_test.go - Unit tests for the Json feeds column encode/decode helpers.
##
*/

package prismaadapter

import (
	"testing"

	db "overdrive/services/race-data-service/resources/db"
	"overdrive/services/race-data-service/src/core/domain"
)

// TestDecodeFeeds proves a stored array is decoded into typed feeds, and that an empty,
// null or malformed column value degrades to an empty non-nil slice (never nil/null).
func TestDecodeFeeds(t *testing.T) {
	got := decodeFeeds(db.JSON(`[{"provider":"f1tv","contentId":"1","channelId":"2","label":"Onboard"}]`))
	if len(got) != 1 || got[0].Provider != "f1tv" || got[0].ContentID != "1" || got[0].ChannelID != "2" || got[0].Label != "Onboard" {
		t.Fatalf("unexpected decoded feeds: %+v", got)
	}
	for name, raw := range map[string]db.JSON{"nil": nil, "empty": db.JSON(``), "null": db.JSON(`null`), "malformed": db.JSON(`{not json`), "object": db.JSON(`{"a":1}`)} {
		got := decodeFeeds(raw)
		if got == nil || len(got) != 0 {
			t.Fatalf("%s: expected empty non-nil slice, got %#v", name, got)
		}
	}
}

// TestEncodeFeeds proves nil encodes to `[]` (matching the schema default) and a typed list
// round-trips through decodeFeeds with omitempty dropping the unused provider fields.
func TestEncodeFeeds(t *testing.T) {
	raw, err := encodeFeeds(nil)
	if err != nil || string(raw) != "[]" {
		t.Fatalf("expected `[]` for nil, got %q err=%v", raw, err)
	}
	if string(emptyFeedsJSON) != "[]" {
		t.Fatalf("emptyFeedsJSON must be `[]`, got %q", emptyFeedsJSON)
	}
	feeds := []domain.Feed{{Provider: domain.FeedProviderYouTube, URL: "https://youtu.be/a"}}
	raw, err = encodeFeeds(feeds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `[{"provider":"youtube","url":"https://youtu.be/a"}]` {
		t.Fatalf("unexpected encoding: %s", raw)
	}
	if got := decodeFeeds(raw); len(got) != 1 || got[0] != feeds[0] {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
