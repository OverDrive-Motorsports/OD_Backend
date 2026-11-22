/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog_repository_integration_test.go - Integration tests of the replay catalog query against Postgres.
##
*/

package prismaadapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	db "overdrive/services/race-data-service/resources/db"
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
	// The schema takes its URL from env(RACE_DATA_DATABASE_URL), which the query engine reads from the
	// environment itself: db.WithDatasourceURL is ignored for such a schema.
	t.Setenv("RACE_DATA_DATABASE_URL", raw)
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

// sessionPrefix returns a session ID prefix no other test uses, so tests share the database
// without cleaning it.
func sessionPrefix(t *testing.T) string {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "it:session:" + hex.EncodeToString(b[:]) + ":"
}

// addTelemetry stores one telemetry sample for the session.
func addTelemetry(t *testing.T, client *db.PrismaClient, sessionID string) {
	t.Helper()
	_, err := client.RaceTelemetrySample.CreateOne(
		db.RaceTelemetrySample.SessionID.Set(sessionID),
		db.RaceTelemetrySample.DriverNumber.Set(1),
		db.RaceTelemetrySample.DateUtc.Set(time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC)),
		db.RaceTelemetrySample.Raw.Set(db.JSON(`{}`)),
	).Exec(context.Background())
	if err != nil {
		t.Fatalf("create telemetry: %v", err)
	}
}

// addLocation stores one location sample for the session.
func addLocation(t *testing.T, client *db.PrismaClient, sessionID string) {
	t.Helper()
	_, err := client.RaceLocationSample.CreateOne(
		db.RaceLocationSample.SessionID.Set(sessionID),
		db.RaceLocationSample.DriverNumber.Set(1),
		db.RaceLocationSample.DateUtc.Set(time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC)),
		db.RaceLocationSample.Raw.Set(db.JSON(`{}`)),
	).Exec(context.Background())
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
}

// TestIntegrationSessionsWithReplay_NeedsTelemetryAndLocation proves the EXISTS query: a
// session is playable only with both telemetry and location samples; one of the two, none,
// or an unknown ID is left out. A candidate ID full of SQL syntax is bound, not interpolated.
func TestIntegrationSessionsWithReplay_NeedsTelemetryAndLocation(t *testing.T) {
	client := integrationClient(t)
	repo := NewReplayCatalogRepository(client)
	prefix := sessionPrefix(t)
	both, telemetryOnly, locationOnly, empty := prefix+"both", prefix+"telemetry", prefix+"location", prefix+"empty"
	addTelemetry(t, client, both)
	addLocation(t, client, both)
	addTelemetry(t, client, telemetryOnly)
	addLocation(t, client, locationOnly)

	found, err := repo.SessionsWithReplay(context.Background(), []string{
		both, telemetryOnly, locationOnly, empty, prefix + `x'); DROP TABLE "RaceTelemetrySample"; --`,
	})
	if err != nil {
		t.Fatalf("SessionsWithReplay: %v", err)
	}
	if _, ok := found[both]; !ok || len(found) != 1 {
		t.Fatalf("found = %v, want only %s", found, both)
	}
}

// TestIntegrationSessionsWithReplay_Batches proves candidates beyond one batch of
// replayCatalogBatchSize are all checked: playable sessions placed in the first, a middle and
// the last batch are all found.
func TestIntegrationSessionsWithReplay_Batches(t *testing.T) {
	client := integrationClient(t)
	repo := NewReplayCatalogRepository(client)
	prefix := sessionPrefix(t)

	total := 2*replayCatalogBatchSize + 3
	ids := make([]string, total)
	for index := range ids {
		ids[index] = fmt.Sprintf("%s%d", prefix, index)
	}
	playable := []string{ids[0], ids[replayCatalogBatchSize+1], ids[total-1]}
	for _, id := range playable {
		addTelemetry(t, client, id)
		addLocation(t, client, id)
	}

	found, err := repo.SessionsWithReplay(context.Background(), ids)
	if err != nil {
		t.Fatalf("SessionsWithReplay: %v", err)
	}
	if len(found) != len(playable) {
		t.Fatalf("found %d sessions, want %d: %v", len(found), len(playable), found)
	}
	for _, id := range playable {
		if _, ok := found[id]; !ok {
			t.Fatalf("%s missing from %v", id, found)
		}
	}
}

// TestIntegrationSessionsWithReplay_NoCandidates proves an empty input answers an empty,
// non-nil set.
func TestIntegrationSessionsWithReplay_NoCandidates(t *testing.T) {
	repo := NewReplayCatalogRepository(integrationClient(t))

	found, err := repo.SessionsWithReplay(context.Background(), nil)
	if err != nil || found == nil || len(found) != 0 {
		t.Fatalf("SessionsWithReplay(nil) = %v, %v; want an empty set", found, err)
	}
}
