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
