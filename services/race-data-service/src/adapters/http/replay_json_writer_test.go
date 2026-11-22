/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_json_writer_test.go - Tests for the streamed replay JSON writer.
##
*/

package httpadapter

import (
	"bytes"
	"encoding/json"
	"testing"

	"overdrive/services/race-data-service/src/core/domain"
)

// TestReplayJSONWriter_RejectsBadSequences proves the writer refuses parts that would produce
// invalid or ambiguous JSON: a dataset going backwards, a driver-scoped part without a driver, a
// track-wide dataset sent twice.
func TestReplayJSONWriter_RejectsBadSequences(t *testing.T) {
	driver := 1
	cases := map[string][]domain.ReplayPart{
		"out of order":          {{Dataset: "position", DriverNumber: &driver, Rows: json.RawMessage("[]")}, {Dataset: "telemetry", DriverNumber: &driver, Rows: json.RawMessage("[]")}},
		"missing driver":        {{Dataset: "telemetry", Rows: json.RawMessage("[]")}},
		"track-wide sent twice": {{Dataset: "weather", Rows: json.RawMessage("[]")}, {Dataset: "weather", Rows: json.RawMessage("[]")}},
	}
	for name, parts := range cases {
		writer := newReplayJSONWriter(&bytes.Buffer{})
		var err error
		for _, part := range parts {
			if err = writer.write(part); err != nil {
				break
			}
		}
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// TestReplayJSONWriter_ValidJSONWithSkippedDatasets proves parts that skip datasets still give
// a valid object holding every key.
func TestReplayJSONWriter_ValidJSONWithSkippedDatasets(t *testing.T) {
	var out bytes.Buffer
	writer := newReplayJSONWriter(&out)
	driver := 7
	if err := writer.write(domain.ReplayPart{Dataset: "lap", DriverNumber: &driver, Rows: json.RawMessage(`[{"lapNumber":1}]`)}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON %s: %v", out.String(), err)
	}
	if len(decoded) != len(domain.ReplayDatasets) || string(decoded["lap"]) != `{"7":[{"lapNumber":1}]}` {
		t.Fatalf("got %s", out.String())
	}
}
