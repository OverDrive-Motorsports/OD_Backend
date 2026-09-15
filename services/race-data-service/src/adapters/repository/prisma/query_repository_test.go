/**
##
## OverDrive 2026
## All Technical rights reserved
##
## query_repository_test.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
##
*/

package prismaadapter

import (
	"errors"
	"fmt"
	"testing"

	db "overdrive/services/race-data-service/resources/db"
)

// TestIsNotFoundErr proves it recognizes db.ErrNotFound (including wrapped via %w) and rejects any
// other error, including nil.
func TestIsNotFoundErr(t *testing.T) {
	if !isNotFoundErr(db.ErrNotFound) {
		t.Fatal("expected db.ErrNotFound to be recognized")
	}
	if !isNotFoundErr(fmt.Errorf("wrapped: %w", db.ErrNotFound)) {
		t.Fatal("expected a wrapped db.ErrNotFound to be recognized via errors.Is semantics")
	}
	if isNotFoundErr(errors.New("some other error")) {
		t.Fatal("expected an unrelated error not to be recognized")
	}
	if isNotFoundErr(nil) {
		t.Fatal("expected nil not to be recognized as ErrNotFound")
	}
}

// TestRawMap proves it unwraps valid Prisma JSON into a map, and falls back to an empty (non-nil)
// map for both an empty value and malformed JSON, rather than panicking or returning nil.
func TestRawMap(t *testing.T) {
	got := rawMap(db.JSON(`{"a":1,"b":"two"}`))
	if got["a"].(float64) != 1 || got["b"] != "two" {
		t.Fatalf("unexpected decoded map: %+v", got)
	}

	empty := rawMap(db.JSON(``))
	if empty == nil || len(empty) != 0 {
		t.Fatalf("expected an empty non-nil map for empty input, got %#v", empty)
	}

	malformed := rawMap(db.JSON(`not-json`))
	if malformed == nil || len(malformed) != 0 {
		t.Fatalf("expected an empty non-nil map for malformed JSON, got %#v", malformed)
	}
}

// TestIntFromAny_QueryRepository covers every supported dynamic-value branch of this package's
// intFromAny (note: unlike usecases.intFromAny, this variant has no string-parsing branch).
func TestIntFromAny_QueryRepository(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{"int", 1, 1},
		{"int32", int32(2), 2},
		{"int64", int64(3), 3},
		{"float64", float64(4.9), 4},
		{"unsupported type falls back to zero", "5", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := intFromAny(tc.value); got != tc.want {
				t.Fatalf("intFromAny(%#v) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}
