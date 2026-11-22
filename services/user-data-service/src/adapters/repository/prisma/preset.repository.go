/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset.repository.go - Prisma-backed storage of user presets.
##
*/

package prismaadapter

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/steebchen/prisma-client-go/runtime/raw"

	db "overdrive/services/user-data-service/resources/db"
	"overdrive/services/user-data-service/src/core/domain"
)

type PresetRepository struct {
	client *db.PrismaClient
}

// NewPresetRepository builds and returns a preset repository with its required dependencies.
func NewPresetRepository(client *db.PrismaClient) *PresetRepository {
	return &PresetRepository{client: client}
}

// isNotFoundErr reports whether err is prisma-client-go's "record not found".
func isNotFoundErr(err error) bool {
	return errors.Is(err, db.ErrNotFound)
}

// ListByUser returns the user's presets, newest first.
func (r *PresetRepository) ListByUser(ctx context.Context, userID string) ([]domain.Preset, error) {
	rows, err := r.client.UserPreset.FindMany(
		db.UserPreset.UserID.Equals(userID),
	).OrderBy(db.UserPreset.CreatedAt.Order(db.SortOrderDesc)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	presets := make([]domain.Preset, 0, len(rows))
	for index := range rows {
		presets = append(presets, mapPreset(&rows[index]))
	}
	return presets, nil
}

// GetByID returns the user's preset; nil when missing or owned by another user.
func (r *PresetRepository) GetByID(ctx context.Context, userID string, presetID string) (*domain.Preset, error) {
	row, err := r.client.UserPreset.FindFirst(
		db.UserPreset.ID.Equals(presetID),
		db.UserPreset.UserID.Equals(userID),
	).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	preset := mapPreset(row)
	return &preset, nil
}

// Create stores a preset in one transaction serialised per user (lockUserPresets): the
// insert only happens while the user owns fewer than MaxPresetsPerUser presets, and the new
// preset becomes the default when requested or when it is the user's first one, clearing the
// previous default. Returns domain.ErrPresetLimitReached when the limit blocked the insert.
func (r *PresetRepository) Create(ctx context.Context, userID string, input domain.NewPreset) (*domain.Preset, error) {
	id, err := newPresetID()
	if err != nil {
		return nil, err
	}
	lock := r.lockUserPresets(userID)
	insert := r.client.Prisma.ExecuteRaw(insertPresetQuery, id, userID, input.Name, input.IsDefault, string(input.Data), domain.MaxPresetsPerUser).Tx()
	clear := r.client.Prisma.ExecuteRaw(clearOtherDefaultsQuery, userID, id).Tx()
	if err := r.client.Prisma.Transaction(lock, insert, clear).Exec(ctx); err != nil {
		return nil, err
	}
	if insert.Result().Count == 0 {
		return nil, domain.ErrPresetLimitReached
	}
	return r.GetByID(ctx, userID, id)
}

// Update applies the changes to the user's preset; nil when missing or not owned. Ownership is
// checked first (the update itself is by primary key); setting the default clears the user's
// other defaults in the same transaction, serialised per user by lockUserPresets. A preset
// deleted concurrently answers nil (404), on either path.
func (r *PresetRepository) Update(ctx context.Context, userID string, presetID string, changes domain.PresetChanges) (*domain.Preset, error) {
	existing, err := r.GetByID(ctx, userID, presetID)
	if err != nil || existing == nil {
		return nil, err
	}
	params := []db.UserPresetSetParam{}
	if changes.Name != nil {
		params = append(params, db.UserPreset.Name.Set(*changes.Name))
	}
	if changes.IsDefault != nil {
		params = append(params, db.UserPreset.IsDefault.Set(*changes.IsDefault))
	}
	if changes.Data != nil {
		params = append(params, db.UserPreset.Data.Set(db.JSON(changes.Data)))
	}
	update := r.client.UserPreset.FindUnique(db.UserPreset.ID.Equals(presetID)).Update(params...)

	if changes.IsDefault == nil || !*changes.IsDefault {
		row, err := update.Exec(ctx)
		if err != nil {
			if isNotFoundErr(err) {
				return nil, nil
			}
			return nil, err
		}
		preset := mapPreset(row)
		return &preset, nil
	}
	lock := r.lockUserPresets(userID)
	clear := r.clearDefaults(userID, presetID)
	updated := update.Tx()
	if err := r.client.Prisma.Transaction(lock, clear, updated).Exec(ctx); err != nil {
		// A failed batch transaction does not surface db.ErrNotFound: check whether the preset
		// vanished in the meantime before reporting an internal error.
		if stillThere, lookupErr := r.GetByID(ctx, userID, presetID); lookupErr == nil && stillThere == nil {
			return nil, nil
		}
		return nil, err
	}
	preset := mapPreset(updated.Result())
	return &preset, nil
}

// Delete removes the user's preset; false when missing or not owned.
func (r *PresetRepository) Delete(ctx context.Context, userID string, presetID string) (bool, error) {
	result, err := r.client.UserPreset.FindMany(
		db.UserPreset.ID.Equals(presetID),
		db.UserPreset.UserID.Equals(userID),
	).Delete().Exec(ctx)
	if err != nil {
		return false, err
	}
	return result.Count > 0, nil
}

// lockUserPresetsQuery takes a transaction-scoped advisory lock keyed on the user ID. Every
// write that depends on the user's other presets (count limit, single default) runs it first,
// so two concurrent requests of one user are serialised instead of both passing the check.
const lockUserPresetsQuery = `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`

// insertPresetQuery inserts the preset only below the per-user limit ($6); it is the default
// when requested ($4) or when it is the user's first preset.
const insertPresetQuery = `INSERT INTO "UserPreset" ("id", "userId", "name", "isDefault", "data", "updatedAt")
SELECT $1::text, $2::text, $3::text,
       $4::boolean OR NOT EXISTS (SELECT 1 FROM "UserPreset" WHERE "userId" = $2::text),
       $5::jsonb, now()
WHERE (SELECT count(*) FROM "UserPreset" WHERE "userId" = $2::text) < $6::int`

// clearOtherDefaultsQuery unsets the user's other defaults, only if the preset $2 was just
// inserted as the default.
const clearOtherDefaultsQuery = `UPDATE "UserPreset" SET "isDefault" = false, "updatedAt" = now()
WHERE "userId" = $1::text AND "isDefault" AND "id" <> $2::text
  AND EXISTS (SELECT 1 FROM "UserPreset" WHERE "id" = $2::text AND "isDefault")`

// lockUserPresets returns the advisory-lock statement (ExecuteRaw: the function returns void,
// which QueryRaw cannot decode).
func (r *PresetRepository) lockUserPresets(userID string) raw.TxExecuteResult {
	return r.client.Prisma.ExecuteRaw(lockUserPresetsQuery, "user-presets:"+userID).Tx()
}

// newPresetID returns a random UUID v4. Prisma's @default(uuid()) is applied by the Prisma
// engine, not by Postgres, so the raw insert has to provide the ID itself.
func newPresetID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// clearDefaults unsets the default flag on the user's presets, except keepID when non-empty.
func (r *PresetRepository) clearDefaults(userID string, keepID string) db.UserPresetManyTxResult {
	where := []db.UserPresetWhereParam{
		db.UserPreset.UserID.Equals(userID),
		db.UserPreset.IsDefault.Equals(true),
	}
	if keepID != "" {
		where = append(where, db.UserPreset.Not(db.UserPreset.ID.Equals(keepID)))
	}
	return r.client.UserPreset.FindMany(where...).Update(db.UserPreset.IsDefault.Set(false)).Tx()
}

// mapPreset converts a stored row into the domain preset.
func mapPreset(row *db.UserPresetModel) domain.Preset {
	return domain.Preset{
		ID:        row.ID,
		Name:      row.Name,
		IsDefault: row.IsDefault,
		Data:      json.RawMessage(row.Data),
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
