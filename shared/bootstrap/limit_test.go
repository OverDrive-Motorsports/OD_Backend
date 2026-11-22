/**
##
## OverDrive 2026
## All Technical rights reserved
##
## limit_test.go - Tests for the bounded upstream response reader.
##
*/

package bootstrap

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// TestLimitResponse_UnderAndAtLimit proves a body up to the limit is read whole.
func TestLimitResponse_UnderAndAtLimit(t *testing.T) {
	for _, body := range []string{"abc", "abcdefghij"} {
		got, err := io.ReadAll(LimitResponse(strings.NewReader(body), 10))
		if err != nil {
			t.Fatalf("%q: err = %v", body, err)
		}
		if string(got) != body {
			t.Fatalf("got %q, want %q", got, body)
		}
	}
}

// TestLimitResponse_OverLimitFails proves one byte over the limit fails with
// ErrResponseTooLarge instead of being silently truncated.
func TestLimitResponse_OverLimitFails(t *testing.T) {
	got, err := io.ReadAll(LimitResponse(strings.NewReader("abcdefghijk"), 10))
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("err = %v, want ErrResponseTooLarge", err)
	}
	if len(got) > 10 {
		t.Fatalf("read %d bytes past the limit", len(got))
	}
}

// TestLimitResponse_JSONDecoderSurfacesError proves a JSON decoder reading an oversized body
// reports ErrResponseTooLarge, which is how the upstream clients use it.
func TestLimitResponse_JSONDecoderSurfacesError(t *testing.T) {
	body := `[` + strings.Repeat(`{"a":1},`, 100) + `{"a":1}]`
	var rows []map[string]any
	err := json.NewDecoder(LimitResponse(strings.NewReader(body), 64)).Decode(&rows)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("err = %v, want ErrResponseTooLarge", err)
	}
}
