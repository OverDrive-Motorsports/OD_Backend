/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset_repository_integration_test.go - Integration tests of the preset repository against Postgres.
##
*/

package prismaadapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	db "overdrive/services/user-data-service/resources/db"
	"overdrive/services/user-data-service/src/core/domain"
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
	// The schema takes its URL from env(USER_DATA_DATABASE_URL), which the query engine reads from the
	// environment itself: db.WithDatasourceURL is ignored for such a schema.
	t.Setenv("USER_DATA_DATABASE_URL", raw)
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

// newUserID returns a user ID no other test uses, so tests share the database without
// cleaning it.
func newUserID(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "it-user-" + hex.EncodeToString(b[:])
}

// presetInput is a valid creation input.
func presetInput(name string, isDefault bool) domain.NewPreset {
	return domain.NewPreset{Name: name, IsDefault: isDefault, Data: json.RawMessage(`{"widgets":[]}`)}
}

// defaultIDs returns the IDs of the user's default presets, read back from the database.
func defaultIDs(t *testing.T, repo *PresetRepository, userID string) []string {
	t.Helper()
	presets, err := repo.ListByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	var ids []string
	for _, preset := range presets {
		if preset.IsDefault {
			ids = append(ids, preset.ID)
		}
	}
	return ids
}

// TestIntegrationPresetCreate_DefaultRules proves the raw insert's default rules: the first
// preset is the default even when not requested, a later preset is not unless requested, and a
// preset created as the default takes the flag from the previous one.
func TestIntegrationPresetCreate_DefaultRules(t *testing.T) {
	repo := NewPresetRepository(integrationClient(t))
	ctx := context.Background()
	userID := newUserID(t)

	first, err := repo.Create(ctx, userID, presetInput("first", false))
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if !first.IsDefault {
		t.Fatal("the first preset must be the default")
	}
	second, err := repo.Create(ctx, userID, presetInput("second", false))
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if second.IsDefault {
		t.Fatal("a later preset must not be the default unless requested")
	}
	third, err := repo.Create(ctx, userID, presetInput("third", true))
	if err != nil {
		t.Fatalf("create third: %v", err)
	}
	if got := defaultIDs(t, repo, userID); len(got) != 1 || got[0] != third.ID {
		t.Fatalf("defaults = %v, want only %s", got, third.ID)
	}
	if string(third.Data) != `{"widgets": []}` && string(third.Data) != `{"widgets":[]}` {
		t.Fatalf("data = %s, want the stored document", third.Data)
	}
}

// TestIntegrationPresetCreate_LimitUnderConcurrency proves the per-user limit holds when
// creations of one user run at the same time on the last free slot: the advisory lock
// serialises them, so exactly one succeeds, the others get ErrPresetLimitReached, the user
// ends with MaxPresetsPerUser presets and one default. Another user is not affected.
func TestIntegrationPresetCreate_LimitUnderConcurrency(t *testing.T) {
	repo := NewPresetRepository(integrationClient(t))
	ctx := context.Background()
	userID := newUserID(t)
	for i := 0; i < domain.MaxPresetsPerUser-1; i++ {
		if _, err := repo.Create(ctx, userID, presetInput("prefill", false)); err != nil {
			t.Fatalf("prefill %d: %v", i, err)
		}
	}
	const attempts = 10

	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Create(ctx, userID, presetInput("concurrent", false))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	created, limited := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			created++
		case errors.Is(err, domain.ErrPresetLimitReached):
			limited++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if created != 1 || limited != attempts-1 {
		t.Fatalf("created=%d limited=%d, want 1 and %d", created, limited, attempts-1)
	}
	presets, err := repo.ListByUser(ctx, userID)
	if err != nil || len(presets) != domain.MaxPresetsPerUser {
		t.Fatalf("stored presets = %d (err %v), want %d", len(presets), err, domain.MaxPresetsPerUser)
	}
	if got := defaultIDs(t, repo, userID); len(got) != 1 {
		t.Fatalf("defaults = %v, want exactly one", got)
	}
	if _, err := repo.Create(ctx, newUserID(t), presetInput("other user", false)); err != nil {
		t.Fatalf("another user's creation failed: %v", err)
	}
}

// TestIntegrationPresetCreate_SingleDefaultUnderConcurrency proves concurrent creations that
// all ask to be the default leave exactly one default.
func TestIntegrationPresetCreate_SingleDefaultUnderConcurrency(t *testing.T) {
	repo := NewPresetRepository(integrationClient(t))
	ctx := context.Background()
	userID := newUserID(t)
	const attempts = 10

	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Create(ctx, userID, presetInput("default", true))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if got := defaultIDs(t, repo, userID); len(got) != 1 {
		t.Fatalf("defaults = %v, want exactly one", got)
	}
}

// TestIntegrationPresetUpdate_SingleDefaultUnderConcurrency proves concurrent updates that
// each set a different preset as the default leave exactly one default, and that unsetting
// the default takes no lock but leaves the user without one.
func TestIntegrationPresetUpdate_SingleDefaultUnderConcurrency(t *testing.T) {
	repo := NewPresetRepository(integrationClient(t))
	ctx := context.Background()
	userID := newUserID(t)
	const count = 10

	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		preset, err := repo.Create(ctx, userID, presetInput("p", false))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		ids = append(ids, preset.ID)
	}

	isDefault := true
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for _, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Update(ctx, userID, id, domain.PresetChanges{IsDefault: &isDefault})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("update: %v", err)
		}
	}
	defaults := defaultIDs(t, repo, userID)
	if len(defaults) != 1 {
		t.Fatalf("defaults = %v, want exactly one", defaults)
	}

	notDefault := false
	if _, err := repo.Update(ctx, userID, defaults[0], domain.PresetChanges{IsDefault: &notDefault}); err != nil {
		t.Fatalf("unset default: %v", err)
	}
	if got := defaultIDs(t, repo, userID); len(got) != 0 {
		t.Fatalf("defaults = %v, want none", got)
	}
}

// TestIntegrationPresetUpdate_ChangesFields proves a partial update writes only the given
// fields and the JSON document is stored as jsonb.
func TestIntegrationPresetUpdate_ChangesFields(t *testing.T) {
	repo := NewPresetRepository(integrationClient(t))
	ctx := context.Background()
	userID := newUserID(t)
	preset, err := repo.Create(ctx, userID, presetInput("before", false))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	name := "after"
	updated, err := repo.Update(ctx, userID, preset.ID, domain.PresetChanges{Name: &name, Data: json.RawMessage(`{"a":1}`)})
	if err != nil || updated == nil {
		t.Fatalf("update: %v %v", updated, err)
	}
	if updated.Name != "after" || !updated.IsDefault {
		t.Fatalf("updated = %+v, want the new name and the default flag kept", updated)
	}
	var data map[string]int
	if err := json.Unmarshal(updated.Data, &data); err != nil || data["a"] != 1 {
		t.Fatalf("data = %s (err %v)", updated.Data, err)
	}
}

// TestIntegrationPreset_OtherUserSeesNothing proves another user's preset is invisible:
// reading, updating and deleting it answer "not found" and leave it intact.
func TestIntegrationPreset_OtherUserSeesNothing(t *testing.T) {
	repo := NewPresetRepository(integrationClient(t))
	ctx := context.Background()
	owner, intruder := newUserID(t), newUserID(t)
	preset, err := repo.Create(ctx, owner, presetInput("mine", false))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if got, err := repo.GetByID(ctx, intruder, preset.ID); err != nil || got != nil {
		t.Fatalf("GetByID by another user = %v, %v; want nil, nil", got, err)
	}
	name := "stolen"
	if got, err := repo.Update(ctx, intruder, preset.ID, domain.PresetChanges{Name: &name}); err != nil || got != nil {
		t.Fatalf("Update by another user = %v, %v; want nil, nil", got, err)
	}
	if deleted, err := repo.Delete(ctx, intruder, preset.ID); err != nil || deleted {
		t.Fatalf("Delete by another user = %v, %v; want false, nil", deleted, err)
	}
	stored, err := repo.GetByID(ctx, owner, preset.ID)
	if err != nil || stored == nil || stored.Name != "mine" {
		t.Fatalf("owner's preset = %v, %v; want it unchanged", stored, err)
	}
	if deleted, err := repo.Delete(ctx, owner, preset.ID); err != nil || !deleted {
		t.Fatalf("Delete by owner = %v, %v; want true, nil", deleted, err)
	}
	if got, err := repo.GetByID(ctx, owner, preset.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = %v, %v; want nil, nil", got, err)
	}
}
