/**
##
## OverDrive 2026
## All Technical rights reserved
##
## catalog_repository_test.go - Package prismaadapter source file for services/championship-service/src/adapters/repository/prisma.
##
*/

package prismaadapter

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	db "overdrive/services/championship-service/resources/db"
)

// TestIsNotFoundErr proves the Prisma "no row matched" sentinel is recognized (including when
// wrapped), and that an unrelated error is not misclassified as a not-found.
func TestIsNotFoundErr(t *testing.T) {
	if !isNotFoundErr(db.ErrNotFound) {
		t.Fatal("expected db.ErrNotFound to be classified as not-found")
	}
	if !isNotFoundErr(errors.Join(errors.New("wrapped"), db.ErrNotFound)) {
		t.Fatal("expected a wrapped db.ErrNotFound to still be classified as not-found")
	}
	if isNotFoundErr(errors.New("some other failure")) {
		t.Fatal("an unrelated error must not be classified as not-found")
	}
}

// TestStringifyAny proves the raw-JSON-to-display-string conversion: nil becomes an empty
// string, a plain string passes through unchanged, and any other JSON-able value is rendered
// via json.Marshal rather than Go's default %v formatting.
func TestStringifyAny(t *testing.T) {
	if got := stringifyAny(nil); got != "" {
		t.Fatalf("nil: got %q, want empty string", got)
	}
	if got := stringifyAny("+1.234"); got != "+1.234" {
		t.Fatalf("string passthrough: got %q, want %q", got, "+1.234")
	}
	if got := stringifyAny(float64(12)); got != "12" {
		t.Fatalf("number: got %q, want %q", got, "12")
	}
	if got := stringifyAny(map[string]any{"a": 1}); got != `{"a":1}` {
		t.Fatalf("object: got %q, want %q", got, `{"a":1}`)
	}
}

// TestRawMap proves rawMap safely unwraps Prisma JSON blobs, returning an empty (non-nil) map
// for both an empty payload and malformed JSON, rather than propagating a decode error.
func TestRawMap(t *testing.T) {
	if got := rawMap(db.JSON("")); len(got) != 0 {
		t.Fatalf("empty payload: got %v, want empty map", got)
	}
	if got := rawMap(db.JSON("not-json")); len(got) != 0 {
		t.Fatalf("malformed payload: got %v, want empty map", got)
	}
	got := rawMap(db.JSON(`{"gap_to_leader":"+1.234"}`))
	if got["gap_to_leader"] != "+1.234" {
		t.Fatalf("valid payload: got %v, want gap_to_leader=+1.234", got)
	}
}

// TestEnrichDriverRow proves the driver-lookup enrichment always sets driver_number, fills in
// driver_name/team_name only when absent from the source row (never overwriting provider data
// already present), and only sets team_color when a non-empty color is known.
func TestEnrichDriverRow(t *testing.T) {
	lookup := map[int]driverLookupRow{
		44: {name: "Lewis Hamilton", teamName: "Mercedes", teamColor: "#00D2BE"},
		1:  {name: "Max Verstappen", teamName: "Red Bull", teamColor: ""},
	}

	t.Run("fills in missing fields from the lookup", func(t *testing.T) {
		item := map[string]any{}
		enrichDriverRow(item, 44, lookup)
		if item["driver_number"] != 44 || item["driver_name"] != "Lewis Hamilton" || item["team_name"] != "Mercedes" || item["team_color"] != "#00D2BE" {
			t.Fatalf("unexpected enrichment: %+v", item)
		}
	})

	t.Run("never overwrites data already present on the row", func(t *testing.T) {
		item := map[string]any{"driver_name": "provider-supplied name", "team_name": "provider-supplied team"}
		enrichDriverRow(item, 44, lookup)
		if item["driver_name"] != "provider-supplied name" || item["team_name"] != "provider-supplied team" {
			t.Fatalf("expected existing fields to be preserved, got %+v", item)
		}
	})

	t.Run("does not set team_color when the lookup color is empty", func(t *testing.T) {
		item := map[string]any{}
		enrichDriverRow(item, 1, lookup)
		if _, ok := item["team_color"]; ok {
			t.Fatalf("expected no team_color key, got %+v", item)
		}
	})

	t.Run("unknown driver number only sets driver_number", func(t *testing.T) {
		item := map[string]any{}
		enrichDriverRow(item, 999, lookup)
		if len(item) != 1 || item["driver_number"] != 999 {
			t.Fatalf("expected only driver_number to be set for an unknown driver, got %+v", item)
		}
	})
}

// TestIntPtrFromInt proves the OpenF1 zero-value convention: 0 is treated as "absent" and
// returns nil, any other value is returned by pointer.
func TestIntPtrFromInt(t *testing.T) {
	if got := intPtrFromInt(0); got != nil {
		t.Fatalf("expected nil for zero, got %v", *got)
	}
	got := intPtrFromInt(7)
	if got == nil || *got != 7 {
		t.Fatalf("expected pointer to 7, got %v", got)
	}
}

// TestTimeValue proves the db.DateTime -> time.Time conversion is a transparent alias
// round-trip (DateTime is a type alias for time.Time upstream).
func TestTimeValue(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	if got := timeValue(now); !got.Equal(now) {
		t.Fatalf("got %v, want %v", got, now)
	}
}

// TestMapEventSummary proves the Prisma EventModel row -> domain.EventSummary mapping pulls
// every optional field through its accessor and carries the championship code passed in
// separately (it isn't stored on the row itself).
func TestMapEventSummary(t *testing.T) {
	officialName := "Formula 1 Test Grand Prix 2026"
	countryName := "Testland"
	countryCode := "TL"
	circuitName := "Test Circuit"
	externalKey := "1234"
	roundNumber := 5
	start := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	row := &db.EventModel{
		InnerEvent: db.InnerEvent{
			ID:             "e1",
			ChampionshipID: "c1",
			SeasonYear:     2026,
			RoundNumber:    &roundNumber,
			Name:           "Test Grand Prix",
			OfficialName:   &officialName,
			Location:       "Test City",
			CountryName:    &countryName,
			CountryCode:    &countryCode,
			CircuitName:    &circuitName,
			ExternalKey:    &externalKey,
			StartTimeUtc:   start,
			EndTimeUtc:     end,
			Status:         db.EventStatusLive,
		},
	}

	got := mapEventSummary(row, "f1-2026")

	if got.ID != "e1" || got.ChampionshipID != "c1" || got.ChampionshipCode != "f1-2026" {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.RoundNumber == nil || *got.RoundNumber != 5 {
		t.Fatalf("unexpected RoundNumber: %v", got.RoundNumber)
	}
	if got.OfficialName != officialName || got.Circuit != circuitName || got.CountryName != countryName || got.CountryCode != countryCode || got.ExternalKey != externalKey {
		t.Fatalf("unexpected optional fields: %+v", got)
	}
	if got.Status != "live" {
		t.Fatalf("unexpected status: %v", got.Status)
	}
	if !got.StartDate.Equal(start) || !got.EndDate.Equal(end) {
		t.Fatalf("unexpected dates: %+v", got)
	}
}

// TestMapEventSummary_MissingRoundNumber proves a nil RoundNumber (never ingested) is preserved
// as nil rather than defaulted to a pointer to zero.
func TestMapEventSummary_MissingRoundNumber(t *testing.T) {
	row := &db.EventModel{InnerEvent: db.InnerEvent{ID: "e1"}}
	got := mapEventSummary(row, "f1-2026")
	if got.RoundNumber != nil {
		t.Fatalf("expected nil RoundNumber, got %v", *got.RoundNumber)
	}
}

// TestMapSessionSummary proves the Prisma SessionModel row -> domain.SessionSummary mapping,
// including the EndedAtUtc optional-pointer conversion (present and absent cases) and that the
// circuit name is carried in from a separately-resolved value rather than the row itself.
func TestMapSessionSummary(t *testing.T) {
	name := "Race"
	externalKey := "9998"
	broadcastURL := "https://example.com/watch"
	start := time.Date(2026, 8, 10, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 10, 16, 0, 0, 0, time.UTC)

	t.Run("session with an end time", func(t *testing.T) {
		row := &db.SessionModel{
			InnerSession: db.InnerSession{
				ID:           "s1",
				EventID:      "e1",
				Type:         db.SessionTypeRace,
				Status:       db.SessionStatusFinished,
				Name:         &name,
				ExternalKey:  &externalKey,
				BroadcastURL: &broadcastURL,
				StartedAtUtc: start,
				EndedAtUtc:   &end,
			},
		}
		got := mapSessionSummary(row, "Test Circuit")
		if got.ID != "s1" || got.EventID != "e1" || got.Type != "race" || got.Status != "finished" {
			t.Fatalf("unexpected identity/status fields: %+v", got)
		}
		if got.Name != name || got.Circuit != "Test Circuit" || got.ExternalKey != externalKey || got.BroadcastURL != broadcastURL {
			t.Fatalf("unexpected optional fields: %+v", got)
		}
		if !got.StartTime.Equal(start) {
			t.Fatalf("unexpected StartTime: %v", got.StartTime)
		}
		if got.EndTime == nil || !got.EndTime.Equal(end) {
			t.Fatalf("expected EndTime %v, got %v", end, got.EndTime)
		}
	})

	t.Run("session still in progress has no end time", func(t *testing.T) {
		row := &db.SessionModel{InnerSession: db.InnerSession{ID: "s1", StartedAtUtc: start}}
		got := mapSessionSummary(row, "")
		if got.EndTime != nil {
			t.Fatalf("expected nil EndTime, got %v", *got.EndTime)
		}
	})
}

// TestSessionMetadata proves the event+session -> API metadata combination pulls meeting/session
// keys and names through their optional accessors, and fixes "provider" to "openf1" (the only
// supported catalog provider today).
func TestSessionMetadata(t *testing.T) {
	meetingKey := "1234"
	sessionKey := "9998"
	countryName := "Testland"
	sessionName := "Race"

	event := &db.EventModel{InnerEvent: db.InnerEvent{
		Name:        "Test Grand Prix",
		SeasonYear:  2026,
		ExternalKey: &meetingKey,
		CountryName: &countryName,
	}}
	session := &db.SessionModel{InnerSession: db.InnerSession{
		ExternalKey: &sessionKey,
		Name:        &sessionName,
	}}

	got := sessionMetadata(event, session)

	want := map[string]any{
		"provider":          "openf1",
		"meeting_name":      "Test Grand Prix",
		"country_name":      countryName,
		"year":              2026,
		"meeting_key":       meetingKey,
		"race_session_name": sessionName,
		"race_session_key":  sessionKey,
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("key %q: got %v, want %v (full: %+v)", k, got[k], v, got)
		}
	}
}

// TestRawMap_RoundTripsRealJSON is a small sanity check that rawMap round-trips genuine
// marshaled JSON (as produced by the ingestion side via jsonValue), not just hand-written
// literals.
func TestRawMap_RoundTripsRealJSON(t *testing.T) {
	encoded, err := json.Marshal(map[string]any{"position": 1, "points": 25.0})
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	got := rawMap(db.JSON(encoded))
	if got["position"] != float64(1) || got["points"] != 25.0 {
		t.Fatalf("unexpected round-tripped payload: %+v", got)
	}
}
