/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_repository_pure_test.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
##
*/

package prismaadapter

import (
	"encoding/json"
	"testing"
)

// TestParseTime covers a valid RFC3339 timestamp, an empty value, and an unparsable string.
func TestParseTime(t *testing.T) {
	got, ok := parseTime("2026-03-08T15:00:00Z")
	if !ok {
		t.Fatal("expected ok=true for a valid RFC3339 string")
	}
	if got.Year() != 2026 || got.Month() != 3 || got.Day() != 8 {
		t.Fatalf("unexpected parsed time: %v", got)
	}

	if _, ok := parseTime(""); ok {
		t.Fatal("expected ok=false for an empty value")
	}
	if _, ok := parseTime("not-a-time"); ok {
		t.Fatal("expected ok=false for an unparsable string")
	}
	if _, ok := parseTime(nil); ok {
		t.Fatal("expected ok=false for a nil value")
	}
}

// TestParseOptionalTime proves it returns a non-nil pointer on success and nil otherwise.
func TestParseOptionalTime(t *testing.T) {
	got := parseOptionalTime("2026-03-08T15:00:00Z")
	if got == nil {
		t.Fatal("expected a non-nil pointer for a valid timestamp")
	}
	if parseOptionalTime("") != nil {
		t.Fatal("expected nil for an empty value")
	}
	if parseOptionalTime("garbage") != nil {
		t.Fatal("expected nil for an unparsable string")
	}
}

// TestIntValue covers every supported dynamic-value branch plus the unparsable-string/unsupported-
// type fallback to zero.
func TestIntValue(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{"int", 1, 1},
		{"int32", int32(2), 2},
		{"int64", int64(3), 3},
		{"float64", float64(4.9), 4},
		{"numeric string with whitespace", " 5 ", 5},
		{"unparsable string", "abc", 0},
		{"unsupported type", true, 0},
		{"nil", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := intValue(tc.value); got != tc.want {
				t.Fatalf("intValue(%#v) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

// TestIntPtr proves a zero result from intValue (either a real zero or an unparsable input) yields
// a nil pointer rather than a pointer-to-zero, since 0 is used as the "absent" sentinel.
func TestIntPtr(t *testing.T) {
	if got := intPtr(5); got == nil || *got != 5 {
		t.Fatalf("intPtr(5) = %v, want pointer to 5", got)
	}
	if got := intPtr(0); got != nil {
		t.Fatalf("intPtr(0) = %v, want nil", got)
	}
	if got := intPtr("not-a-number"); got != nil {
		t.Fatalf("intPtr(unparsable) = %v, want nil", got)
	}
}

// TestFloatPtr covers every supported dynamic-value branch and the unsupported-type/unparsable
// fallback to nil.
func TestFloatPtr(t *testing.T) {
	if got := floatPtr(1.5); got == nil || *got != 1.5 {
		t.Fatalf("floatPtr(1.5) = %v, want pointer to 1.5", got)
	}
	if got := floatPtr(float32(2.5)); got == nil || *got != 2.5 {
		t.Fatalf("floatPtr(float32(2.5)) = %v, want pointer to 2.5", got)
	}
	if got := floatPtr(3); got == nil || *got != 3 {
		t.Fatalf("floatPtr(3) = %v, want pointer to 3", got)
	}
	if got := floatPtr(int64(4)); got == nil || *got != 4 {
		t.Fatalf("floatPtr(int64(4)) = %v, want pointer to 4", got)
	}
	if got := floatPtr(" 5.5 "); got == nil || *got != 5.5 {
		t.Fatalf("floatPtr(\" 5.5 \") = %v, want pointer to 5.5", got)
	}
	if got := floatPtr("not-a-number"); got != nil {
		t.Fatalf("floatPtr(unparsable) = %v, want nil", got)
	}
	if got := floatPtr(true); got != nil {
		t.Fatalf("floatPtr(unsupported type) = %v, want nil", got)
	}
}

// TestBoolPtr covers the bool and string branches, and the unparsable/unsupported fallback to nil.
func TestBoolPtr(t *testing.T) {
	if got := boolPtr(true); got == nil || !*got {
		t.Fatalf("boolPtr(true) = %v, want pointer to true", got)
	}
	if got := boolPtr(" true "); got == nil || !*got {
		t.Fatalf("boolPtr(\" true \") = %v, want pointer to true", got)
	}
	if got := boolPtr("not-a-bool"); got != nil {
		t.Fatalf("boolPtr(unparsable) = %v, want nil", got)
	}
	if got := boolPtr(1); got != nil {
		t.Fatalf("boolPtr(unsupported type) = %v, want nil", got)
	}
}

// TestStringPtr proves an empty result from stringify yields a nil pointer, and a non-empty value
// yields a pointer to the trimmed string.
func TestStringPtr(t *testing.T) {
	if got := stringPtr(" hello "); got == nil || *got != "hello" {
		t.Fatalf("stringPtr(\" hello \") = %v, want pointer to \"hello\"", got)
	}
	if got := stringPtr(""); got != nil {
		t.Fatalf("stringPtr(\"\") = %v, want nil", got)
	}
	if got := stringPtr(nil); got != nil {
		t.Fatalf("stringPtr(nil) = %v, want nil", got)
	}
}

// TestStringify covers every supported dynamic-value branch, including the default
// fmt.Sprintf/TrimSpace fallback.
func TestStringify(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"nil", nil, ""},
		{"string with whitespace", "  hi  ", "hi"},
		{"int", 42, "42"},
		{"int64", int64(43), "43"},
		{"float64", float64(3.9), "3"},
		{"bool fallback", true, "true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stringify(tc.value); got != tc.want {
				t.Fatalf("stringify(%#v) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

// TestJSONValue proves a Go value round-trips through jsonValue into a valid db.JSON payload, and
// that an unmarshalable value (e.g. a channel) surfaces a marshal error.
func TestJSONValue(t *testing.T) {
	got, err := jsonValue(map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("expected valid JSON, got error decoding: %v", err)
	}
	if decoded["a"].(float64) != 1 {
		t.Fatalf("unexpected round-tripped value: %+v", decoded)
	}

	if _, err := jsonValue(make(chan int)); err == nil {
		t.Fatal("expected an error for an unmarshalable value")
	}
}
