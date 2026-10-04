/**
##
## OverDrive 2026
## All Technical rights reserved
##
## feed.entity.go - Video feed descriptor and its validation rules (shared shape with championship-service).
##
*/

package domain

import (
	"fmt"
	"net/url"
	"strings"
)

// Feed providers accepted by ValidateFeeds. Anything else is rejected.
const (
	FeedProviderF1TV    = "f1tv"
	FeedProviderYouTube = "youtube"
	FeedProviderHLS     = "hls"
)

// Length limits enforced by ValidateFeeds.
const (
	MaxFeedsPerList     = 20
	MaxFeedIDLength     = 64
	MaxFeedURLLength    = 2048
	MaxFeedLabelLength  = 80
	hlsPlaylistFileExt  = ".m3u8"
	requiredFeedScheme  = "https"
	feedFieldProvider   = "provider"
	feedFieldContentID  = "contentId"
	feedFieldChannelID  = "channelId"
	feedFieldURL        = "url"
	feedFieldLabel      = "label"
	feedFieldWholeEntry = ""
)

// youtubeHosts lists the hostnames a youtube feed URL may point to.
var youtubeHosts = map[string]struct{}{
	"youtube.com":     {},
	"www.youtube.com": {},
	"m.youtube.com":   {},
	"youtu.be":        {},
}

// Feed is a stable video feed descriptor. The backend never stores a playable
// DRM URL or a user token: an f1tv feed only carries the F1 TV contentId
// (session) + channelId (world feed, pit lane, driver onboard...) that the
// client resolves with the user's own F1 TV account, while youtube/hls feeds
// are plain public URLs. This struct is the same in championship-service
// (global session feeds) and race-data-service (per-driver onboard feeds) —
// keep both copies identical.
type Feed struct {
	Provider  string `json:"provider"`
	ContentID string `json:"contentId,omitempty"`
	ChannelID string `json:"channelId,omitempty"`
	URL       string `json:"url,omitempty"`
	Label     string `json:"label,omitempty"`
}

// ValidateFeeds enforces the feed list contract: known provider, per-provider
// required/forbidden fields, https URL rules, length limits, at most
// MaxFeedsPerList entries and no duplicate. Every failure is returned as
// ErrInvalidFeed wrapped with a message naming the failing index and field so
// the HTTP layer can classify it with errors.Is and surface a 400.
func ValidateFeeds(feeds []Feed) error {
	if len(feeds) > MaxFeedsPerList {
		return fmt.Errorf("%w: feeds: at most %d entries allowed, got %d", ErrInvalidFeed, MaxFeedsPerList, len(feeds))
	}
	seen := make(map[string]int, len(feeds))
	for index, feed := range feeds {
		if err := validateFeed(index, feed); err != nil {
			return err
		}
		key := feedIdentity(feed)
		if previous, duplicate := seen[key]; duplicate {
			return feedError(index, feedFieldWholeEntry, fmt.Sprintf("duplicates feeds[%d]", previous))
		}
		seen[key] = index
	}
	return nil
}

// validateFeed checks one entry against the provider-specific rules.
func validateFeed(index int, feed Feed) error {
	if len(feed.Label) > MaxFeedLabelLength {
		return feedError(index, feedFieldLabel, fmt.Sprintf("must be at most %d characters", MaxFeedLabelLength))
	}
	switch feed.Provider {
	case FeedProviderF1TV:
		if feed.URL != "" {
			return feedError(index, feedFieldURL, "must not be set for provider f1tv")
		}
		if err := validateFeedID(index, feedFieldContentID, feed.ContentID); err != nil {
			return err
		}
		return validateFeedID(index, feedFieldChannelID, feed.ChannelID)
	case FeedProviderYouTube, FeedProviderHLS:
		if feed.ContentID != "" {
			return feedError(index, feedFieldContentID, "must not be set for provider "+feed.Provider)
		}
		if feed.ChannelID != "" {
			return feedError(index, feedFieldChannelID, "must not be set for provider "+feed.Provider)
		}
		return validateFeedURL(index, feed.Provider, feed.URL)
	default:
		return feedError(index, feedFieldProvider, "must be one of f1tv, youtube, hls")
	}
}

// validateFeedID checks a non-empty, bounded-length F1 TV identifier.
func validateFeedID(index int, field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return feedError(index, field, "is required for provider f1tv")
	}
	if len(value) > MaxFeedIDLength {
		return feedError(index, field, fmt.Sprintf("must be at most %d characters", MaxFeedIDLength))
	}
	return nil
}

// validateFeedURL checks an absolute https URL and the provider-specific host/path rule.
func validateFeedURL(index int, provider string, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return feedError(index, feedFieldURL, "is required for provider "+provider)
	}
	if len(raw) > MaxFeedURLLength {
		return feedError(index, feedFieldURL, fmt.Sprintf("must be at most %d characters", MaxFeedURLLength))
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return feedError(index, feedFieldURL, "must be an absolute https URL")
	}
	if !strings.EqualFold(parsed.Scheme, requiredFeedScheme) {
		return feedError(index, feedFieldURL, "must use the https scheme")
	}
	switch provider {
	case FeedProviderYouTube:
		if _, ok := youtubeHosts[strings.ToLower(parsed.Hostname())]; !ok {
			return feedError(index, feedFieldURL, "must point to youtube.com or youtu.be")
		}
	case FeedProviderHLS:
		if !strings.HasSuffix(strings.ToLower(parsed.Path), hlsPlaylistFileExt) {
			return feedError(index, feedFieldURL, "must end with "+hlsPlaylistFileExt)
		}
	}
	return nil
}

// feedIdentity builds the key used for duplicate detection: provider + url for
// URL-based providers, provider + contentId + channelId for f1tv.
func feedIdentity(feed Feed) string {
	if feed.Provider == FeedProviderF1TV {
		return feed.Provider + "|" + feed.ContentID + "|" + feed.ChannelID
	}
	return feed.Provider + "|" + feed.URL
}

// feedError wraps ErrInvalidFeed with the offending index/field and reason.
func feedError(index int, field string, reason string) error {
	if field == feedFieldWholeEntry {
		return fmt.Errorf("%w: feeds[%d] %s", ErrInvalidFeed, index, reason)
	}
	return fmt.Errorf("%w: feeds[%d].%s %s", ErrInvalidFeed, index, field, reason)
}
