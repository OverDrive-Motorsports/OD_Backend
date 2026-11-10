/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset.repository.go - Persistence port for user presets.
##
*/

package ports

import (
	"context"

	"overdrive/services/user-data-service/src/core/domain"
)

// PresetRepository stores presets. Every method is scoped to userID: a preset owned by another
// user behaves exactly like a missing one (nil / false), so ownership can't be bypassed.
type PresetRepository interface {
	// ListByUser returns the user's presets, newest first.
	ListByUser(ctx context.Context, userID string) ([]domain.Preset, error)
	// GetByID returns the user's preset, or nil when missing or owned by someone else.
	GetByID(ctx context.Context, userID string, presetID string) (*domain.Preset, error)
	// Create stores a preset atomically with the per-user rules: at most MaxPresetsPerUser
	// presets (domain.ErrPresetLimitReached otherwise).
	Create(ctx context.Context, userID string, input domain.NewPreset) (*domain.Preset, error)
	// Update applies the changes and returns the stored preset, or nil when missing or not
	// owned. Setting IsDefault to true clears the flag on the user's other presets in the same
	// transaction.
	Update(ctx context.Context, userID string, presetID string, changes domain.PresetChanges) (*domain.Preset, error)
	// Delete removes the preset; false when missing or not owned.
	Delete(ctx context.Context, userID string, presetID string) (bool, error)
}
