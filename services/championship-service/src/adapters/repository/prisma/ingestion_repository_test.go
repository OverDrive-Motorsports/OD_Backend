/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_repository_test.go - Package prismaadapter source file for services/championship-service/src/adapters/repository/prisma.
##
*/

package prismaadapter

import (
	"testing"
	"time"

	db "overdrive/services/championship-service/resources/db"
)

// TestInferEventStatus proves the three-way status derivation: before start is "scheduled",
// after end is "finished", and anything in between (inclusive of the boundaries themselves) is
// "live".
func TestInferEventStatus(t *testing.T) {
	start := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		now  time.Time
		want db.EventStatus
	}{
		{"before start", start.Add(-time.Hour), db.EventStatusScheduled},
		{"at start", start, db.EventStatusLive},
		{"between start and end", start.Add(30 * time.Minute), db.EventStatusLive},
		{"at end", end, db.EventStatusLive},
		{"after end", end.Add(time.Hour), db.EventStatusFinished},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inferEventStatus(start, end, tc.now); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestInferSessionStatus mirrors TestInferEventStatus for the session-status variant, which
// uses the same three-way boundary logic against a different enum type.
func TestInferSessionStatus(t *testing.T) {
	start := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	if got := inferSessionStatus(start, end, start.Add(-time.Hour)); got != db.SessionStatusScheduled {
		t.Fatalf("before start: got %v, want scheduled", got)
	}
	if got := inferSessionStatus(start, end, start.Add(time.Minute)); got != db.SessionStatusLive {
		t.Fatalf("during: got %v, want live", got)
	}
	if got := inferSessionStatus(start, end, end.Add(time.Minute)); got != db.SessionStatusFinished {
		t.Fatalf("after end: got %v, want finished", got)
	}
}

// TestMapSessionType proves the OpenF1 session_type string is normalized case-insensitively,
// "qualifying" is aliased to the storage-level "quali" value, and anything unrecognized falls
// back to "practice" rather than erroring.
func TestMapSessionType(t *testing.T) {
	cases := map[string]db.SessionType{
		"Race":        db.SessionTypeRace,
		"  race  ":    db.SessionTypeRace,
		"SPRINT":      db.SessionTypeSprint,
		"quali":       db.SessionTypeQuali,
		"Qualifying":  db.SessionTypeQuali,
		"Practice 1":  db.SessionTypePractice,
		"":            db.SessionTypePractice,
		"unknown-foo": db.SessionTypePractice,
	}
	for input, want := range cases {
		if got := mapSessionType(input); got != want {
			t.Fatalf("mapSessionType(%q) = %v, want %v", input, got, want)
		}
	}
}

// TestParseTime proves parseTime only accepts RFC3339 and normalizes the result to UTC, and
// that both a missing and a malformed value produce an error rather than a zero-value silently.
func TestParseTime(t *testing.T) {
	got, err := parseTime("2026-08-10T10:00:00+02:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("got %v, want %v in UTC", got, want)
	}

	if _, err := parseTime(nil); err == nil {
		t.Fatal("expected an error for a missing time value")
	}
	if _, err := parseTime("not-a-timestamp"); err == nil {
		t.Fatal("expected an error for a malformed time value")
	}
}

// TestIntValue proves the numeric-payload coercion handles every JSON-decodable numeric shape
// plus numeric strings, and falls back to zero for anything else (including a malformed
// string) rather than erroring - this function has no error return, so zero is the documented
// "absent" sentinel used throughout the ingestion mapping.
func TestIntValue(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{"int", 7, 7},
		{"int32", int32(7), 7},
		{"int64", int64(7), 7},
		{"float64", float64(7.9), 7},
		{"numeric string", "42", 42},
		{"string with whitespace", " 42 ", 42},
		{"malformed string", "not-a-number", 0},
		{"nil", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := intValue(tc.value); got != tc.want {
				t.Fatalf("intValue(%v) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

// TestStringify proves every supported dynamic value renders to a trimmed string
// representation, and nil renders to an empty string.
func TestStringify(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"nil", nil, ""},
		{"string with whitespace", "  hello  ", "hello"},
		{"int", 7, "7"},
		{"int32", int32(7), "7"},
		{"int64", int64(7), "7"},
		{"float64", float64(7), "7"},
		{"bool falls back to fmt", true, "true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stringify(tc.value); got != tc.want {
				t.Fatalf("stringify(%v) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

// TestJsonValue proves jsonValue is a thin json.Marshal wrapper into the Prisma JSON type.
func TestJsonValue(t *testing.T) {
	got, err := jsonValue(map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("got %s, want %s", got, `{"a":1}`)
	}

	if _, err := jsonValue(make(chan int)); err == nil {
		t.Fatal("expected an error for an unmarshalable value")
	}
}

// TestFallbackString proves fallbackString returns the fallback exactly when stringify would
// have produced an empty string, and the real value otherwise.
func TestFallbackString(t *testing.T) {
	if got := fallbackString(nil, "default"); got != "default" {
		t.Fatalf("got %q, want %q", got, "default")
	}
	if got := fallbackString("actual", "default"); got != "actual" {
		t.Fatalf("got %q, want %q", got, "actual")
	}
}

// TestStringPtr proves stringPtr converts an empty/absent value to nil (rather than a pointer
// to an empty string) and a present value to a pointer to its string form.
func TestStringPtr(t *testing.T) {
	if got := stringPtr(nil); got != nil {
		t.Fatalf("expected nil, got %q", *got)
	}
	got := stringPtr("hello")
	if got == nil || *got != "hello" {
		t.Fatalf("expected pointer to %q, got %v", "hello", got)
	}
}

// TestIntPtr proves intPtr treats zero (intValue's absent sentinel) as nil, and any non-zero
// value as a pointer.
func TestIntPtr(t *testing.T) {
	if got := intPtr(nil); got != nil {
		t.Fatalf("expected nil, got %v", *got)
	}
	if got := intPtr(0); got != nil {
		t.Fatalf("expected nil for zero, got %v", *got)
	}
	got := intPtr(5)
	if got == nil || *got != 5 {
		t.Fatalf("expected pointer to 5, got %v", got)
	}
}

// TestFloatPtr proves floatPtr accepts every numeric/string shape and returns nil for anything
// else (unlike intPtr/intValue, there is no numeric "absent" sentinel here - only type
// mismatch/parse failure yields nil).
func TestFloatPtr(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  *float64
	}{
		{"float64", float64(1.5), floatp(1.5)},
		{"float32", float32(1.5), floatp(1.5)},
		{"int", 2, floatp(2)},
		{"int32", int32(2), floatp(2)},
		{"int64", int64(2), floatp(2)},
		{"numeric string", "3.5", floatp(3.5)},
		{"malformed string", "n/a", nil},
		{"unsupported type", true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := floatPtr(tc.value)
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("floatPtr(%v) = %v, want %v", tc.value, got, tc.want)
			}
			if got != nil && *got != *tc.want {
				t.Fatalf("floatPtr(%v) = %v, want %v", tc.value, *got, *tc.want)
			}
		})
	}
}

func floatp(v float64) *float64 { return &v }

// TestTimePtr proves timePtr treats a zero time.Time as "absent" (nil), matching the same
// absent-sentinel convention as intPtr/intValue.
func TestTimePtr(t *testing.T) {
	if got := timePtr(time.Time{}); got != nil {
		t.Fatalf("expected nil for the zero time, got %v", *got)
	}
	value := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	got := timePtr(value)
	if got == nil || !time.Time(*got).Equal(value) {
		t.Fatalf("expected pointer to %v, got %v", value, got)
	}
}

// TestColorPtr proves colorPtr normalizes a bare hex value by prefixing "#" (provider data is
// inconsistent about including it), leaves an already-prefixed value untouched, and returns nil
// for an absent value.
func TestColorPtr(t *testing.T) {
	if got := colorPtr(nil); got != nil {
		t.Fatalf("expected nil, got %q", *got)
	}
	got := colorPtr("00D2BE")
	if got == nil || *got != "#00D2BE" {
		t.Fatalf("expected #00D2BE, got %v", got)
	}
	got = colorPtr("#00D2BE")
	if got == nil || *got != "#00D2BE" {
		t.Fatalf("expected #00D2BE unchanged, got %v", got)
	}
}

// TestSessionBroadcastURL proves the placeholder broadcast URL is only built for a non-empty
// session key, and embeds that key verbatim.
func TestSessionBroadcastURL(t *testing.T) {
	if got := sessionBroadcastURL(""); got != "" {
		t.Fatalf("expected empty string for an empty session key, got %q", got)
	}
	if got := sessionBroadcastURL("  "); got != "" {
		t.Fatalf("expected empty string for a whitespace-only session key, got %q", got)
	}
	got := sessionBroadcastURL("9998")
	if got != "https://www.youtube.com/watch?v=dQw4w9WgXcQ&session=9998" {
		t.Fatalf("unexpected broadcast URL: %q", got)
	}
}

// TestTeamExternalKey proves the fallback chain: an explicit team_external_key wins, otherwise
// the team name is slugified, and an entirely absent name falls back to a fixed placeholder.
func TestTeamExternalKey(t *testing.T) {
	if got := teamExternalKey(map[string]any{"team_external_key": "red-bull-racing", "team_name": "Ignored"}); got != "red-bull-racing" {
		t.Fatalf("expected explicit key to win, got %q", got)
	}
	if got := teamExternalKey(map[string]any{"team_name": "Red Bull Racing"}); got != "red-bull-racing" {
		t.Fatalf("expected slugified team name, got %q", got)
	}
	if got := teamExternalKey(map[string]any{}); got != "unknown-team" {
		t.Fatalf("expected the fixed placeholder, got %q", got)
	}
}

// TestDriverExternalKey proves the fallback chain: explicit key wins over driver number, which
// wins over driver code, which wins over a fixed placeholder when nothing is present.
func TestDriverExternalKey(t *testing.T) {
	if got := driverExternalKey(map[string]any{"driver_external_key": "63", "driver_number": 44, "driver_code": "HAM"}); got != "63" {
		t.Fatalf("expected explicit key to win, got %q", got)
	}
	if got := driverExternalKey(map[string]any{"driver_number": 44, "driver_code": "HAM"}); got != "44" {
		t.Fatalf("expected driver number to win over code, got %q", got)
	}
	if got := driverExternalKey(map[string]any{"driver_code": "HAM"}); got != "ham" {
		t.Fatalf("expected lowercased driver code, got %q", got)
	}
	if got := driverExternalKey(map[string]any{}); got != "unknown-driver" {
		t.Fatalf("expected the fixed placeholder, got %q", got)
	}
}
