/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset_repository_test.go - Unit tests for the preset row mapping.
##
*/

package prismaadapter

import (
	"regexp"
	"strings"
	"testing"
	"time"

	db "overdrive/services/user-data-service/resources/db"
)

// TestMapPreset proves a stored row maps field by field, the JSON document passing through
// unchanged; the userId column is never exposed.
func TestMapPreset(t *testing.T) {
	created := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	row := &db.UserPresetModel{InnerUserPreset: db.InnerUserPreset{
		ID: "p1", UserID: "alice", Name: "Race view", IsDefault: true,
		Data: db.JSON(`{"widgets":[{"type":"video"}]}`), CreatedAt: created, UpdatedAt: created.Add(time.Hour),
	}}
	got := mapPreset(row)
	if got.ID != "p1" || got.Name != "Race view" || !got.IsDefault || string(got.Data) != `{"widgets":[{"type":"video"}]}` ||
		!got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(created.Add(time.Hour)) {
		t.Fatalf("unexpected mapping: %+v", got)
	}
}

// TestPresetSQL pins the raw statements behind the per-user rules: a transaction-scoped
// advisory lock, an insert guarded by the limit that also makes the first preset the default,
// and a clear of the other defaults conditioned on the new preset being the default. Every
// value is a bind parameter, never interpolated.
func TestPresetSQL(t *testing.T) {
	if lockUserPresetsQuery != `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))` {
		t.Fatalf("unexpected lock statement: %s", lockUserPresetsQuery)
	}
	for _, want := range []string{
		`INSERT INTO "UserPreset" ("id", "userId", "name", "isDefault", "data", "updatedAt")`,
		`$4::boolean OR NOT EXISTS (SELECT 1 FROM "UserPreset" WHERE "userId" = $2::text)`,
		`$5::jsonb`,
		`WHERE (SELECT count(*) FROM "UserPreset" WHERE "userId" = $2::text) < $6::int`,
	} {
		if !strings.Contains(insertPresetQuery, want) {
			t.Fatalf("insert query misses %q:\n%s", want, insertPresetQuery)
		}
	}
	for _, want := range []string{`SET "isDefault" = false`, `"id" <> $2::text`, `EXISTS (SELECT 1 FROM "UserPreset" WHERE "id" = $2::text AND "isDefault")`} {
		if !strings.Contains(clearOtherDefaultsQuery, want) {
			t.Fatalf("clear query misses %q:\n%s", want, clearOtherDefaultsQuery)
		}
	}
}

// TestNewPresetID proves IDs are distinct RFC 4122 version-4 UUIDs (the raw insert bypasses
// Prisma's @default(uuid())).
func TestNewPresetID(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for index := 0; index < 100; index++ {
		id, err := newPresetID()
		if err != nil || !pattern.MatchString(id) || seen[id] {
			t.Fatalf("bad or duplicate id %q (err=%v)", id, err)
		}
		seen[id] = true
	}
}
