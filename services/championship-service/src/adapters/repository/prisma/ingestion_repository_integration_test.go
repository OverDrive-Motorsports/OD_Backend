/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_repository_integration_test.go - Integration tests of the championship ingestion against Postgres.
##
*/

package prismaadapter

import (
	"context"
	"crypto/rand"
	"math/big"
	"net/url"
	"os"
	"strings"
	"testing"

	db "overdrive/services/championship-service/resources/db"
	contracts "overdrive/shared/contracts/ingestion"
)

// integrationClient connects to the database named by INTEGRATION_DATABASE_URL, or skips the
// test when it is unset (plain `go test`). The database name must end with "_test": the tests
// write rows, so a development database is refused, and the database actually reached is
// checked after connecting.
func integrationClient(t *testing.T) *db.PrismaClient {
	t.Helper()
	raw := os.Getenv("INTEGRATION_DATABASE_URL")
	if raw == "" {
		t.Skip("INTEGRATION_DATABASE_URL is not set: run tools/integration-test.sh")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !strings.HasSuffix(strings.TrimPrefix(parsed.Path, "/"), "_test") {
		t.Fatal("INTEGRATION_DATABASE_URL must name a database ending with _test")
	}
	// The schema takes its URL from env(CHAMPIONSHIP_DATABASE_URL), which the query engine reads from the
	// environment itself: db.WithDatasourceURL is ignored for such a schema.
	t.Setenv("CHAMPIONSHIP_DATABASE_URL", raw)
	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Prisma.Disconnect(); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	})
	var current []struct {
		Name string `json:"name"`
	}
	if err := client.Prisma.QueryRaw(`SELECT current_database()::text AS name`).Exec(context.Background(), &current); err != nil ||
		len(current) != 1 || !strings.HasSuffix(current[0].Name, "_test") {
		t.Fatalf("connected to %v (err %v), want a database ending with _test", current, err)
	}
	return client
}

// randomKey returns an OpenF1-like key no other test uses, so tests share the database without
// cleaning it.
func randomKey(t *testing.T) int {
	t.Helper()
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000_000))
	if err != nil {
		t.Fatalf("rand: %v", err)
	}
	return int(n.Int64()) + 1_000_000_000
}

// eventRow is an event_catalog row as ingestion-service maps it.
func eventRow(meetingKey int, name string, season int) map[string]any {
	return map[string]any{
		"meeting_key":    meetingKey,
		"meeting_name":   name,
		"season_year":    season,
		"location":       "Sakhir",
		"country_name":   "Bahrain",
		"circuit_name":   "Bahrain International Circuit",
		"start_time_utc": "2026-03-01T10:00:00Z",
		"end_time_utc":   "2026-03-03T18:00:00Z",
	}
}

// sessionRow is a session_catalog row: it carries the meeting key but neither the meeting name
// nor the season.
func sessionRow(meetingKey int, sessionKey int, name string) map[string]any {
	return map[string]any{
		"meeting_key":    meetingKey,
		"session_key":    sessionKey,
		"session_name":   name,
		"session_type":   "Race",
		"start_time_utc": "2025-12-31T23:00:00Z",
		"end_time_utc":   "2026-01-01T01:00:00Z",
	}
}

// storeBatch stores one batch whose datasets are given as name/rows pairs, in order.
func storeBatch(t *testing.T, repo *IngestionRepository, datasets ...contracts.Dataset) {
	t.Helper()
	batch := contracts.Batch{BatchID: "it", Provider: "openf1", Datasets: datasets}
	if err := repo.StoreBatch(context.Background(), batch); err != nil {
		t.Fatalf("StoreBatch: %v", err)
	}
}

// fetchEvent reads the stored event of a meeting key.
func fetchEvent(t *testing.T, client *db.PrismaClient, meetingKey int) *db.EventModel {
	t.Helper()
	event, err := client.Event.FindUnique(db.Event.ID.Equals(contracts.EventID("openf1", meetingKey))).Exec(context.Background())
	if err != nil {
		t.Fatalf("find event %d: %v", meetingKey, err)
	}
	return event
}

// TestIntegrationStoreBatch_SessionsDoNotOverwriteEvent proves the create-only upsert of the
// event behind a session: the sessions of a meeting, stored after its event in the same batch
// and again on a later re-ingestion, leave the event's real name and season untouched (a
// session row would otherwise rename it "Race" and move it to the session's year).
func TestIntegrationStoreBatch_SessionsDoNotOverwriteEvent(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	meetingKey, sessionKey := randomKey(t), randomKey(t)

	storeBatch(t, repo,
		contracts.Dataset{Name: "event_catalog", Rows: []map[string]any{eventRow(meetingKey, "Bahrain Grand Prix", 2026)}},
		contracts.Dataset{Name: "session_catalog", Rows: []map[string]any{sessionRow(meetingKey, sessionKey, "Race")}},
	)
	storeBatch(t, repo,
		contracts.Dataset{Name: "session_catalog", Rows: []map[string]any{sessionRow(meetingKey, sessionKey, "Race")}},
	)

	event := fetchEvent(t, client, meetingKey)
	if event.Name != "Bahrain Grand Prix" || event.SeasonYear != 2026 || event.Location != "Sakhir" {
		t.Fatalf("event = name %q season %d location %q, want the event_catalog values", event.Name, event.SeasonYear, event.Location)
	}
	session, err := client.Session.FindUnique(db.Session.ID.Equals(contracts.SessionID("openf1", sessionKey))).Exec(context.Background())
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if session.EventID != event.ID {
		t.Fatalf("session event = %q, want %q", session.EventID, event.ID)
	}
}

// TestIntegrationStoreBatch_PlaceholderEventThenRealEvent proves a session ingested before its
// meeting creates a placeholder event (named after the session, season of its start), which
// the later event_catalog then replaces with the real values, keeping the same ID.
func TestIntegrationStoreBatch_PlaceholderEventThenRealEvent(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	meetingKey, sessionKey := randomKey(t), randomKey(t)

	storeBatch(t, repo, contracts.Dataset{Name: "session_catalog", Rows: []map[string]any{sessionRow(meetingKey, sessionKey, "Race")}})
	placeholder := fetchEvent(t, client, meetingKey)
	if placeholder.Name != "Race" || placeholder.SeasonYear != 2025 {
		t.Fatalf("placeholder = name %q season %d, want the session name and its start year", placeholder.Name, placeholder.SeasonYear)
	}

	storeBatch(t, repo, contracts.Dataset{Name: "event_catalog", Rows: []map[string]any{eventRow(meetingKey, "Bahrain Grand Prix", 2026)}})
	event := fetchEvent(t, client, meetingKey)
	if event.ID != placeholder.ID || event.Name != "Bahrain Grand Prix" || event.SeasonYear != 2026 {
		t.Fatalf("event = id %q name %q season %d, want the real values on the same ID", event.ID, event.Name, event.SeasonYear)
	}
}

// TestIntegrationStoreBatch_SessionReingestionUpdatesInPlace proves sessions themselves are
// full upserts: re-ingesting a session updates its row (same deterministic ID) instead of
// adding a second one.
func TestIntegrationStoreBatch_SessionReingestionUpdatesInPlace(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	meetingKey, sessionKey := randomKey(t), randomKey(t)

	storeBatch(t, repo, contracts.Dataset{Name: "session_catalog", Rows: []map[string]any{sessionRow(meetingKey, sessionKey, "Race")}})
	storeBatch(t, repo, contracts.Dataset{Name: "session_catalog", Rows: []map[string]any{sessionRow(meetingKey, sessionKey, "Sprint")}})

	sessions, err := client.Session.FindMany(db.Session.EventID.Equals(contracts.EventID("openf1", meetingKey))).Exec(context.Background())
	if err != nil {
		t.Fatalf("find sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != contracts.SessionID("openf1", sessionKey) {
		t.Fatalf("sessions = %+v, want the single deterministic row", sessions)
	}
	if name, ok := sessions[0].Name(); !ok || name != "Sprint" {
		t.Fatalf("session name = %q (set %v), want the re-ingested value", name, ok)
	}
}
