/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_merge_repository_integration_test.go - Integration tests proving live batches never duplicate samples on Postgres.
##
*/

package prismaadapter

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	db "overdrive/services/race-data-service/resources/db"
	contracts "overdrive/shared/contracts/ingestion"
)

// liveBatch builds a merge batch of one dataset.
func liveBatch(sessionKey int, name string, rows ...map[string]any) contracts.Batch {
	return contracts.Batch{
		BatchID: "it", Provider: "openf1", WriteMode: contracts.WriteModeMerge,
		Context:  contracts.Context{SessionKey: sessionKey, Provider: "openf1"},
		Datasets: []contracts.Dataset{{Name: name, TargetService: "race-data-service", RowCount: len(rows), Rows: rows}},
	}
}

// liveSessionKey returns a session key no other test uses.
func liveSessionKey(t *testing.T) int {
	t.Helper()
	n, err := rand.Int(rand.Reader, big.NewInt(1<<30))
	if err != nil {
		t.Fatal(err)
	}
	return int(n.Int64()) + 1
}

// liveRow returns a row every dataset accepts, at the given second of the session.
func liveRow(second int) map[string]any {
	at := time.Date(2026, 3, 1, 15, 0, second, 123987000, time.UTC).Format(time.RFC3339Nano)
	return map[string]any{
		"driver_number": 44, "sample_time_utc": at, "date_utc": at, "started_at_utc": at,
		"lap_number": second + 1, "stint_number": second + 1, "position": 1, "speed_kph": 300, "throttle_pct": 99.5,
		"gap_to_leader": 12.345, "message": fmt.Sprintf("EVENT %d", second), "category": "Flag", "compound": "SOFT", "air_temp_c": 21.5,
		"recording_url": "https://example.com/a.mp3", "x": 1.5,
	}
}

// queryText runs a query returning one text column named v.
func queryText(t *testing.T, client *db.PrismaClient, query string, sessionKey int) []string {
	t.Helper()
	var rows []struct {
		V string `json:"v"`
	}
	if err := client.Prisma.QueryRaw(query, contracts.SessionID("openf1", sessionKey)).Exec(context.Background(), &rows); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.V)
	}
	return values
}

// countRows counts the rows of a table for a session.
func countRows(t *testing.T, client *db.PrismaClient, table string, sessionKey int) int {
	t.Helper()
	var count int
	values := queryText(t, client, fmt.Sprintf(`SELECT count(*)::text AS v FROM "%s" WHERE "sessionId" = $1::text`, table), sessionKey)
	if _, err := fmt.Sscan(values[0], &count); err != nil {
		t.Fatal(err)
	}
	return count
}

// TestIntegrationMerge_OverlapNeverDuplicates proves, on every dataset of a live batch, that two
// overlapping batches store the union of their rows, and that another session is not affected.
func TestIntegrationMerge_OverlapNeverDuplicates(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	tables := map[string]string{"lap_timing": "RaceLap"}
	for name, table := range mergeTables {
		tables[name] = table.table
	}
	for name, table := range tables {
		key, other := liveSessionKey(t), liveSessionKey(t)
		for _, batch := range []contracts.Batch{
			liveBatch(other, name, liveRow(1)),
			liveBatch(key, name, liveRow(1), liveRow(2)),
			liveBatch(key, name, liveRow(2), liveRow(3)),
			liveBatch(key, name, liveRow(2), liveRow(3)),
		} {
			if _, err := repo.MergeBatch(context.Background(), batch); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		if got := countRows(t, client, table, key); got != 3 {
			t.Fatalf("%s: %d rows, want 3", name, got)
		}
		if got := countRows(t, client, table, other); got != 1 {
			t.Fatalf("%s: other session has %d rows, want 1", name, got)
		}
	}
}

// TestIntegrationMerge_AfterReplace proves a live batch overlapping a manual ingestion adds nothing
// for the samples already stored by Prisma, whatever the engine did with their microseconds.
func TestIntegrationMerge_AfterReplace(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	for name, table := range mergeTables {
		key := liveSessionKey(t)
		batch := liveBatch(key, name, liveRow(1), liveRow(2))
		batch.WriteMode = ""
		if err := repo.StoreBatch(context.Background(), batch); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := repo.MergeBatch(context.Background(), liveBatch(key, name, liveRow(1), liveRow(2), liveRow(3))); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := countRows(t, client, table.table, key); got != 3 {
			t.Fatalf("%s: %d rows, want 3", name, got)
		}
	}
}

// TestIntegrationMerge_Concurrent proves the advisory lock: the same batch sent eight times at
// once (several live jobs or retries) stores each sample once.
func TestIntegrationMerge_Concurrent(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	key := liveSessionKey(t)
	rows := make([]map[string]any, 0, 50)
	for second := range 50 {
		rows = append(rows, liveRow(second))
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.MergeBatch(context.Background(), liveBatch(key, "telemetry", rows...))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := countRows(t, client, "RaceTelemetrySample", key); got != 50 {
		t.Fatalf("%d rows, want 50", got)
	}
}

// TestIntegrationMerge_LargeBatch proves a batch spanning several chunks is stored whole.
func TestIntegrationMerge_LargeBatch(t *testing.T) {
	client := integrationClient(t)
	key := liveSessionKey(t)
	rows := make([]map[string]any, 0, 2*mergeChunkRows+5)
	for second := range 2*mergeChunkRows + 5 {
		rows = append(rows, liveRow(second))
	}
	if _, err := NewIngestionRepository(client).MergeBatch(context.Background(), liveBatch(key, "location", rows...)); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, client, "RaceLocationSample", key); got != 2*mergeChunkRows+5 {
		t.Fatalf("%d rows", got)
	}
}

// TestIntegrationMerge_ReportsNewRaceControl proves only the events actually added are reported,
// including one without a driver, and that the same text at another time is another event.
func TestIntegrationMerge_ReportsNewRaceControl(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	key := liveSessionKey(t)
	first, second := liveRow(1), liveRow(2)
	delete(first, "driver_number")
	second["message"] = first["message"]
	added, err := repo.MergeBatch(context.Background(), liveBatch(key, "race_control", first))
	if err != nil || len(added) != 1 || added[0]["message"] != "EVENT 1" || added[0]["date_utc"] != first["date_utc"] {
		t.Fatalf("added=%v err=%v", added, err)
	}
	added, err = repo.MergeBatch(context.Background(), liveBatch(key, "race_control", first, second))
	if err != nil || len(added) != 1 || added[0]["date_utc"] != second["date_utc"] {
		t.Fatalf("added=%v err=%v", added, err)
	}
}

// TestIntegrationMerge_UpdatesChangingRows proves a stint and a pit stop are updated in place when
// the provider completes them, while an existing sample is kept as stored.
func TestIntegrationMerge_UpdatesChangingRows(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	key := liveSessionKey(t)
	for _, lapEnd := range []int{10, 15} {
		row := liveRow(1)
		row["lap_end"], row["pit_duration_sec"], row["position"] = lapEnd, float64(lapEnd), lapEnd
		for _, name := range []string{"stints", "pit_stops", "position"} {
			if _, err := repo.MergeBatch(context.Background(), liveBatch(key, name, row)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
	for query, want := range map[string]string{
		`SELECT "lapEnd"::text AS v FROM "RaceStint" WHERE "sessionId" = $1::text`:            "15",
		`SELECT "pitDurationSec"::text AS v FROM "RacePitStop" WHERE "sessionId" = $1::text`:  "15",
		`SELECT "position"::text AS v FROM "RacePositionSample" WHERE "sessionId" = $1::text`: "10",
	} {
		if values := queryText(t, client, query, key); len(values) != 1 || values[0] != want {
			t.Fatalf("%s: %v, want %s", query, values, want)
		}
	}
}

// TestIntegrationGapsKeepTheirDecimals proves a gap of 12.345 s is stored as "12.345" by both a
// manual (replace) and a live (merge) ingestion.
func TestIntegrationGapsKeepTheirDecimals(t *testing.T) {
	client := integrationClient(t)
	repo := NewIngestionRepository(client)
	replace, merge := liveSessionKey(t), liveSessionKey(t)
	batch := liveBatch(replace, "intervals", liveRow(1))
	batch.WriteMode = ""
	if err := repo.StoreBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.MergeBatch(context.Background(), liveBatch(merge, "intervals", liveRow(1))); err != nil {
		t.Fatal(err)
	}
	for _, key := range []int{replace, merge} {
		if values := queryText(t, client, `SELECT "gapToLeader" AS v FROM "RaceIntervalSample" WHERE "sessionId" = $1::text`, key); len(values) != 1 || values[0] != "12.345" {
			t.Fatalf("gap = %v", values)
		}
	}
}

// TestIntegrationMerge_UnknownDataset proves a dataset that cannot be merged is refused.
func TestIntegrationMerge_UnknownDataset(t *testing.T) {
	if _, err := NewIngestionRepository(integrationClient(t)).MergeBatch(context.Background(), liveBatch(1, "overtakes", liveRow(1))); err == nil {
		t.Fatal("expected an error")
	}
}
