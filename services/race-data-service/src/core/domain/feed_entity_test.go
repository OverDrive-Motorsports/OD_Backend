/**
##
## OverDrive 2026
## All Technical rights reserved
##
## feed_entity_test.go - Unit tests for ValidateFeeds, one case per validation rule.
##
*/

package domain

import (
	"errors"
	"strings"
	"testing"
)

func validF1TVFeed() Feed {
	return Feed{Provider: FeedProviderF1TV, ContentID: "1000005432", ChannelID: "1017", Label: "Onboard"}
}

func validYouTubeFeed() Feed {
	return Feed{Provider: FeedProviderYouTube, URL: "https://www.youtube.com/watch?v=abc", Label: "Highlights"}
}

func validHLSFeed() Feed {
	return Feed{Provider: FeedProviderHLS, URL: "https://cdn.example.com/demo/index.m3u8", Label: "Demo"}
}

// assertInvalidFeed checks err wraps ErrInvalidFeed and that its message names the expected
// index/field fragment, so a client can locate the offending entry.
func assertInvalidFeed(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected ErrInvalidFeed containing %q, got nil", want)
	}
	if !errors.Is(err, ErrInvalidFeed) {
		t.Fatalf("expected ErrInvalidFeed, got %v", err)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected message to contain %q, got %q", want, err.Error())
	}
}

// TestValidateFeeds_AcceptsEveryProviderAndEmptyList proves a well-formed feed of each
// provider passes, and that an empty or nil list is valid (it clears the list).
func TestValidateFeeds_AcceptsEveryProviderAndEmptyList(t *testing.T) {
	if err := ValidateFeeds([]Feed{validF1TVFeed(), validYouTubeFeed(), validHLSFeed()}); err != nil {
		t.Fatalf("expected valid feeds, got %v", err)
	}
	if err := ValidateFeeds(nil); err != nil {
		t.Fatalf("expected nil list to be valid, got %v", err)
	}
	if err := ValidateFeeds([]Feed{}); err != nil {
		t.Fatalf("expected empty list to be valid, got %v", err)
	}
}

// TestValidateFeeds_RejectsUnknownProvider proves the provider enum is enforced, including
// case variants and an empty provider, and names the index of the bad entry.
func TestValidateFeeds_RejectsUnknownProvider(t *testing.T) {
	for _, provider := range []string{"", "vimeo", "F1TV", "YouTube"} {
		err := ValidateFeeds([]Feed{validHLSFeed(), {Provider: provider, URL: "https://x.example/a.m3u8"}})
		assertInvalidFeed(t, err, "feeds[1].provider")
	}
}

// TestValidateFeeds_F1TVRequiresContentIDAndChannelID proves both identifiers are mandatory
// (whitespace-only counts as missing) for an f1tv feed.
func TestValidateFeeds_F1TVRequiresContentIDAndChannelID(t *testing.T) {
	missingContent := validF1TVFeed()
	missingContent.ContentID = "  "
	assertInvalidFeed(t, ValidateFeeds([]Feed{missingContent}), "feeds[0].contentId")

	missingChannel := validF1TVFeed()
	missingChannel.ChannelID = ""
	assertInvalidFeed(t, ValidateFeeds([]Feed{missingChannel}), "feeds[0].channelId")
}

// TestValidateFeeds_F1TVForbidsURL proves an f1tv feed carrying a url is rejected — the
// backend must never accept a playable/signed F1 TV URL.
func TestValidateFeeds_F1TVForbidsURL(t *testing.T) {
	feed := validF1TVFeed()
	feed.URL = "https://f1tv.formula1.com/manifest.mpd?token=abc"
	assertInvalidFeed(t, ValidateFeeds([]Feed{feed}), "feeds[0].url")
}

// TestValidateFeeds_F1TVIdentifierLengthLimit proves contentId/channelId are capped at
// MaxFeedIDLength characters.
func TestValidateFeeds_F1TVIdentifierLengthLimit(t *testing.T) {
	long := strings.Repeat("a", MaxFeedIDLength+1)
	feed := validF1TVFeed()
	feed.ContentID = long
	assertInvalidFeed(t, ValidateFeeds([]Feed{feed}), "feeds[0].contentId")

	feed = validF1TVFeed()
	feed.ChannelID = long
	assertInvalidFeed(t, ValidateFeeds([]Feed{feed}), "feeds[0].channelId")

	feed = validF1TVFeed()
	feed.ContentID = strings.Repeat("a", MaxFeedIDLength)
	if err := ValidateFeeds([]Feed{feed}); err != nil {
		t.Fatalf("expected exactly %d characters to be accepted, got %v", MaxFeedIDLength, err)
	}
}

// TestValidateFeeds_URLProvidersRequireURL proves youtube and hls feeds must carry a url.
func TestValidateFeeds_URLProvidersRequireURL(t *testing.T) {
	assertInvalidFeed(t, ValidateFeeds([]Feed{{Provider: FeedProviderYouTube}}), "feeds[0].url")
	assertInvalidFeed(t, ValidateFeeds([]Feed{{Provider: FeedProviderHLS, URL: " "}}), "feeds[0].url")
}

// TestValidateFeeds_URLProvidersForbidF1TVIdentifiers proves contentId/channelId are rejected
// on youtube and hls feeds.
func TestValidateFeeds_URLProvidersForbidF1TVIdentifiers(t *testing.T) {
	feed := validYouTubeFeed()
	feed.ContentID = "123"
	assertInvalidFeed(t, ValidateFeeds([]Feed{feed}), "feeds[0].contentId")

	feed = validHLSFeed()
	feed.ChannelID = "1017"
	assertInvalidFeed(t, ValidateFeeds([]Feed{feed}), "feeds[0].channelId")
}

// TestValidateFeeds_URLMustBeAbsoluteHTTPS proves relative URLs, http, other schemes, and
// unparsable values are rejected.
func TestValidateFeeds_URLMustBeAbsoluteHTTPS(t *testing.T) {
	for _, raw := range []string{
		"/relative/index.m3u8",
		"cdn.example.com/index.m3u8",
		"http://cdn.example.com/index.m3u8",
		"ftp://cdn.example.com/index.m3u8",
		"https://",
		"://bad",
	} {
		err := ValidateFeeds([]Feed{{Provider: FeedProviderHLS, URL: raw}})
		assertInvalidFeed(t, err, "feeds[0].url")
	}
}

// TestValidateFeeds_URLLengthLimit proves a url longer than MaxFeedURLLength is rejected.
func TestValidateFeeds_URLLengthLimit(t *testing.T) {
	raw := "https://cdn.example.com/" + strings.Repeat("a", MaxFeedURLLength) + ".m3u8"
	assertInvalidFeed(t, ValidateFeeds([]Feed{{Provider: FeedProviderHLS, URL: raw}}), "feeds[0].url")
}

// TestValidateFeeds_YouTubeHostAllowList proves only the known YouTube hosts are accepted
// (case-insensitively) and any other host is rejected.
func TestValidateFeeds_YouTubeHostAllowList(t *testing.T) {
	for _, raw := range []string{
		"https://youtube.com/watch?v=a",
		"https://www.youtube.com/watch?v=a",
		"https://m.youtube.com/watch?v=a",
		"https://youtu.be/a",
		"https://WWW.YOUTUBE.COM/watch?v=a",
	} {
		if err := ValidateFeeds([]Feed{{Provider: FeedProviderYouTube, URL: raw}}); err != nil {
			t.Fatalf("expected %q to be accepted, got %v", raw, err)
		}
	}
	for _, raw := range []string{
		"https://vimeo.com/123",
		"https://youtube.com.evil.example/watch?v=a",
		"https://notyoutube.com/watch?v=a",
	} {
		assertInvalidFeed(t, ValidateFeeds([]Feed{{Provider: FeedProviderYouTube, URL: raw}}), "feeds[0].url")
	}
}

// TestValidateFeeds_HLSMustEndWithM3U8 proves the hls path extension rule, including that
// a query string after the playlist does not count and that the check is case-insensitive.
func TestValidateFeeds_HLSMustEndWithM3U8(t *testing.T) {
	if err := ValidateFeeds([]Feed{{Provider: FeedProviderHLS, URL: "https://cdn.example.com/a/INDEX.M3U8?token=1"}}); err != nil {
		t.Fatalf("expected upper-case extension with query to be accepted, got %v", err)
	}
	for _, raw := range []string{
		"https://cdn.example.com/a/index.mpd",
		"https://cdn.example.com/a/index",
		"https://cdn.example.com/?file=index.m3u8",
	} {
		assertInvalidFeed(t, ValidateFeeds([]Feed{{Provider: FeedProviderHLS, URL: raw}}), "feeds[0].url")
	}
}

// TestValidateFeeds_LabelLengthLimit proves label is optional but capped at MaxFeedLabelLength.
func TestValidateFeeds_LabelLengthLimit(t *testing.T) {
	feed := validHLSFeed()
	feed.Label = ""
	if err := ValidateFeeds([]Feed{feed}); err != nil {
		t.Fatalf("expected empty label to be accepted, got %v", err)
	}
	feed.Label = strings.Repeat("l", MaxFeedLabelLength+1)
	assertInvalidFeed(t, ValidateFeeds([]Feed{feed}), "feeds[0].label")
}

// TestValidateFeeds_MaxEntries proves exactly MaxFeedsPerList distinct feeds pass and one more
// is rejected before any per-entry validation runs.
func TestValidateFeeds_MaxEntries(t *testing.T) {
	feeds := make([]Feed, 0, MaxFeedsPerList+1)
	for i := 0; i < MaxFeedsPerList; i++ {
		feeds = append(feeds, Feed{Provider: FeedProviderF1TV, ContentID: "c", ChannelID: strings.Repeat("x", i+1)})
	}
	if err := ValidateFeeds(feeds); err != nil {
		t.Fatalf("expected %d feeds to be accepted, got %v", MaxFeedsPerList, err)
	}
	feeds = append(feeds, Feed{Provider: "bogus"})
	err := ValidateFeeds(feeds)
	assertInvalidFeed(t, err, "at most 20 entries")
	if strings.Contains(err.Error(), "provider") {
		t.Fatalf("expected the size check to run before per-entry validation, got %q", err.Error())
	}
}

// TestValidateFeeds_RejectsDuplicates proves duplicate detection keys on provider+url for
// URL providers and provider+contentId+channelId for f1tv, naming both indexes, while the
// same identifiers under different providers are not considered duplicates.
func TestValidateFeeds_RejectsDuplicates(t *testing.T) {
	err := ValidateFeeds([]Feed{validYouTubeFeed(), validHLSFeed(), {Provider: FeedProviderYouTube, URL: validYouTubeFeed().URL, Label: "Other label"}})
	assertInvalidFeed(t, err, "feeds[2] duplicates feeds[0]")

	err = ValidateFeeds([]Feed{validF1TVFeed(), {Provider: FeedProviderF1TV, ContentID: "1000005432", ChannelID: "1017"}})
	assertInvalidFeed(t, err, "feeds[1] duplicates feeds[0]")

	// Same contentId but a different channelId is a different feed.
	other := validF1TVFeed()
	other.ChannelID = "1018"
	if err := ValidateFeeds([]Feed{validF1TVFeed(), other}); err != nil {
		t.Fatalf("expected distinct channelId to be accepted, got %v", err)
	}
}
