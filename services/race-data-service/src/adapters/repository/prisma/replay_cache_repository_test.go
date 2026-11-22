/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_cache_repository_test.go - Tests for the replay cache and race control notification SQL and helpers.
##
*/

package prismaadapter

import (
	"strings"
	"testing"
	"time"
)

// TestReplayCacheQueries pins the raw SQL of the replay cache: only complete full-mode archives
// are served (newest first), chunks come back in write order without their rows, chunk ids and
// rows are read as text, and every value is a bound parameter.
func TestReplayCacheQueries(t *testing.T) {
	cases := map[string][]string{
		findCompleteArchiveQuery: {`"sessionId" = $1`, `"mode" = 'full'`, `("metadata"->>'complete') = 'true'`, `ORDER BY "generatedAt" DESC LIMIT 1`},
		listArchiveChunksQuery:   {`"id"::text AS "id"`, `"archiveId" = $1`, `ORDER BY "chunkIndex"`},
		chunkRowsQuery:           {`"rows"::text AS "rows"`, `"id" = $1::bigint`},
		completeArchiveQuery:     {`'{"complete":true}'::jsonb`, `WHERE "id" = $1 AND NOT EXISTS`, `other."sessionId" = $2`, `(other."metadata"->>'complete') = 'true'`},
		lockSessionArchivesQuery: {`pg_advisory_xact_lock(hashtextextended('replay-archive:' || $1::text, 0))`},
		deleteStaleRecordings:    {`"sessionId" = $1 AND "id" <> $2`, `IS DISTINCT FROM 'true'`, `interval '1 hour'`},
		deleteArchiveQuery:       {`DELETE FROM "RaceArchive" WHERE "id" = $1`},
		deleteSessionArchives:    {`DELETE FROM "RaceArchive" WHERE "sessionId" = $1`},
	}
	for query, fragments := range cases {
		for _, fragment := range fragments {
			if !strings.Contains(query, fragment) {
				t.Errorf("query %q lacks %q", query, fragment)
			}
		}
	}
	if strings.Contains(completeArchiveQuery, "DELETE") || strings.Contains(deleteStaleRecordings, `= 'true'`) {
		t.Error("completing a recording must never delete a complete archive")
	}
	if strings.Contains(listArchiveChunksQuery, `"rows"`) {
		t.Error("chunk listing must not load the rows")
	}
}

// TestCompactJSON proves a chunk read back from jsonb (spaces after ':' and ',') is returned
// compact, and that a non-JSON value is an error.
func TestCompactJSON(t *testing.T) {
	got, err := compactJSON(`[{"x": 1, "speed": 300}, {"x": 2, "speed": 310}]`)
	if err != nil || string(got) != `[{"x":1,"speed":300},{"x":2,"speed":310}]` {
		t.Fatalf("got %s err=%v", got, err)
	}
	if _, err := compactJSON(`not json`); err == nil {
		t.Fatal("compactJSON accepted invalid JSON")
	}
}

// TestRaceControlNotificationQueries pins the notification SQL: rows are stamped and compared
// with the database clock only, the window and age are bound parameters, and id/events are read
// as text.
func TestRaceControlNotificationQueries(t *testing.T) {
	if !strings.Contains(insertRaceControlNotificationQuery, `VALUES ($1::text, $2::jsonb, now())`) {
		t.Errorf("insert = %q", insertRaceControlNotificationQuery)
	}
	for _, fragment := range []string{`"id"::text AS "id"`, `"events"::text AS "events"`, `"createdAt" > now() - make_interval(secs => $1::int)`, `ORDER BY "id"`} {
		if !strings.Contains(listRecentRaceControlQuery, fragment) {
			t.Errorf("list query lacks %q", fragment)
		}
	}
	if !strings.Contains(deleteOldRaceControlQuery, `"createdAt" < now() - make_interval(secs => $1::int)`) {
		t.Errorf("delete = %q", deleteOldRaceControlQuery)
	}
}

// TestDecodeRaceControlNotifications proves raw rows decode into notifications (text id parsed,
// events JSON decoded) and that a malformed id or payload is an error, not a silent skip.
func TestDecodeRaceControlNotifications(t *testing.T) {
	type row = struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionId"`
		Events    string `json:"events"`
	}
	got, err := decodeRaceControlNotifications([]row{{ID: "42", SessionID: "openf1:session:9998", Events: `[{"message":"red flag"}]`}})
	if err != nil || len(got) != 1 || got[0].ID != 42 || got[0].SessionID != "openf1:session:9998" || len(got[0].Events) != 1 || got[0].Events[0].Message != "red flag" {
		t.Fatalf("got %+v err=%v", got, err)
	}
	if _, err := decodeRaceControlNotifications([]row{{ID: "x", Events: `[]`}}); err == nil {
		t.Fatal("bad id accepted")
	}
	if _, err := decodeRaceControlNotifications([]row{{ID: "1", Events: `{`}}); err == nil {
		t.Fatal("bad events accepted")
	}
}

// TestReplayDriverNumbersQuery pins the driver listing: every driver-scoped table is unioned on
// the bound session id.
func TestReplayDriverNumbersQuery(t *testing.T) {
	for _, table := range []string{"RaceTelemetrySample", "RacePositionSample", "TeamRadioMessage", "RacePitStop", "RaceStint", "RaceLap"} {
		if !strings.Contains(replayDriverNumbersQuery, `FROM "`+table+`" WHERE "sessionId" = $1`) {
			t.Errorf("driver query lacks %s", table)
		}
	}
}

// TestWholeSeconds proves durations are bound as whole seconds, rounded up.
func TestWholeSeconds(t *testing.T) {
	for d, want := range map[time.Duration]int{15 * time.Second: 15, 1500 * time.Millisecond: 2, 5 * time.Minute: 300} {
		if got := wholeSeconds(d); got != want {
			t.Errorf("wholeSeconds(%s) = %d, want %d", d, got, want)
		}
	}
}
