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
	"encoding/json"
	"errors"
	"time"

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

// CountByUser returns how many presets the user owns (bounded by MaxPresetsPerUser, so loading
// the IDs is cheap).
func (r *PresetRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	rows, err := r.client.UserPreset.FindMany(db.UserPreset.UserID.Equals(userID)).Select(db.UserPreset.ID.Field()).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

// Create stores a preset; a default preset clears the user's previous default atomically.
func (r *PresetRepository) Create(ctx context.Context, userID string, input domain.NewPreset) (*domain.Preset, error) {
	create := r.client.UserPreset.CreateOne(
		db.UserPreset.UserID.Set(userID),
		db.UserPreset.Name.Set(input.Name),
		db.UserPreset.Data.Set(db.JSON(input.Data)),
		db.UserPreset.IsDefault.Set(input.IsDefault),
	)
	if !input.IsDefault {
		row, err := create.Exec(ctx)
		if err != nil {
			return nil, err
		}
		preset := mapPreset(row)
		return &preset, nil
	}
	clear := r.clearDefaults(userID, "")
	created := create.Tx()
	if err := r.client.Prisma.Transaction(clear, created).Exec(ctx); err != nil {
		return nil, err
	}
	preset := mapPreset(created.Result())
	return &preset, nil
}

// Update applies the changes to the user's preset; nil when missing or not owned. Ownership is
// checked first (the update itself is by primary key); setting the default clears the user's
// other defaults in the same transaction.
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
	clear := r.clearDefaults(userID, presetID)
	updated := update.Tx()
	if err := r.client.Prisma.Transaction(clear, updated).Exec(ctx); err != nil {
		if isNotFoundErr(err) {
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
