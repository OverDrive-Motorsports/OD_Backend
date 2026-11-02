/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset.usecase.go - Application logic for user presets.
##
*/

package usecases

import (
	"context"

	"overdrive/services/user-data-service/src/core/domain"
	"overdrive/services/user-data-service/src/core/ports"
)

type PresetUseCase struct {
	repository ports.PresetRepository
}

// NewPresetUseCase builds and returns a preset use case with its required dependencies.
func NewPresetUseCase(repository ports.PresetRepository) *PresetUseCase {
	return &PresetUseCase{repository: repository}
}

// List returns the user's presets, newest first, never nil.
func (u *PresetUseCase) List(ctx context.Context, userID string) ([]domain.Preset, error) {
	presets, err := u.repository.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if presets == nil {
		presets = []domain.Preset{}
	}
	return presets, nil
}

// Get returns one of the user's presets, or ErrPresetNotFound.
func (u *PresetUseCase) Get(ctx context.Context, userID string, presetID string) (*domain.Preset, error) {
	preset, err := u.repository.GetByID(ctx, userID, presetID)
	if err != nil {
		return nil, err
	}
	if preset == nil {
		return nil, domain.ErrPresetNotFound
	}
	return preset, nil
}

// Create validates the payload, enforces MaxPresetsPerUser, and stores the preset. A user's
// first preset always becomes the default, whatever the payload says, so a user with presets
// always starts with one default.
func (u *PresetUseCase) Create(ctx context.Context, userID string, input domain.NewPreset) (*domain.Preset, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	count, err := u.repository.CountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= domain.MaxPresetsPerUser {
		return nil, domain.ErrPresetLimitReached
	}
	if count == 0 {
		input.IsDefault = true
	}
	return u.repository.Create(ctx, userID, input)
}

// Update validates and applies a partial update, or returns ErrPresetNotFound.
func (u *PresetUseCase) Update(ctx context.Context, userID string, presetID string, changes domain.PresetChanges) (*domain.Preset, error) {
	if err := changes.Validate(); err != nil {
		return nil, err
	}
	preset, err := u.repository.Update(ctx, userID, presetID, changes)
	if err != nil {
		return nil, err
	}
	if preset == nil {
		return nil, domain.ErrPresetNotFound
	}
	return preset, nil
}

// Delete removes the preset, or returns ErrPresetNotFound. Deleting the default preset does
// not promote another one: the client picks the next default explicitly.
func (u *PresetUseCase) Delete(ctx context.Context, userID string, presetID string) error {
	deleted, err := u.repository.Delete(ctx, userID, presetID)
	if err != nil {
		return err
	}
	if !deleted {
		return domain.ErrPresetNotFound
	}
	return nil
}
