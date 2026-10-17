/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog_repository_test.go - Unit tests for the replay catalog SQL builder.
##
*/

package prismaadapter

import (
	"context"
	"strings"
	"testing"
)

// TestSessionsWithReplayQuery proves the query binds exactly n text placeholders (no value is
// ever interpolated) and requires both a telemetry and a location sample per candidate.
func TestSessionsWithReplayQuery(t *testing.T) {
	query := sessionsWithReplayQuery(3)
	if !strings.Contains(query, `VALUES ($1::text), ($2::text), ($3::text)`) || strings.Contains(query, "$4") {
		t.Fatalf("unexpected placeholders: %s", query)
	}
	for _, want := range []string{
		`EXISTS (SELECT 1 FROM "RaceTelemetrySample" t WHERE t."sessionId" = candidate.id)`,
		`AND EXISTS (SELECT 1 FROM "RaceLocationSample" l WHERE l."sessionId" = candidate.id)`,
		`AS "sessionId"`,
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected %s in %s", want, query)
		}
	}
}

// TestSessionsWithReplay_NoCandidate proves an empty candidate list answers an empty set
// without touching the database (nil client would panic otherwise).
func TestSessionsWithReplay_NoCandidate(t *testing.T) {
	got, err := NewReplayCatalogRepository(nil).SessionsWithReplay(context.Background(), nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("expected an empty set, got %#v err=%v", got, err)
	}
}
