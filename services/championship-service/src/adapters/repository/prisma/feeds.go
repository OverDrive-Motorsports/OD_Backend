/**
##
## OverDrive 2026
## All Technical rights reserved
##
## feeds.go - Encodes/decodes the Json "feeds" column into typed domain.Feed values.
##
*/

package prismaadapter

import (
	"encoding/json"
	"fmt"

	db "overdrive/services/championship-service/resources/db"
	"overdrive/services/championship-service/src/core/domain"
)

// emptyFeedsJSON is the value written into the Json column when a row is created:
// an empty array, matching the schema default so the API never emits null.
var emptyFeedsJSON = db.JSON(`[]`)

// decodeFeeds converts the raw Json column into a typed list. A missing, null or
// malformed value degrades to an empty (non-nil) slice rather than an error, so a
// read endpoint always answers `[]` — the column is only ever written through
// encodeFeeds after domain.ValidateFeeds, so malformed content is not expected.
func decodeFeeds(raw db.JSON) []domain.Feed {
	if len(raw) == 0 {
		return []domain.Feed{}
	}
	var feeds []domain.Feed
	if err := json.Unmarshal([]byte(raw), &feeds); err != nil || feeds == nil {
		return []domain.Feed{}
	}
	return feeds
}

// encodeFeeds serialises a typed list for the Json column; nil becomes `[]`.
func encodeFeeds(feeds []domain.Feed) (db.JSON, error) {
	if feeds == nil {
		feeds = []domain.Feed{}
	}
	payload, err := json.Marshal(feeds)
	if err != nil {
		return nil, fmt.Errorf("encode feeds: %w", err)
	}
	return db.JSON(payload), nil
}
