/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset.service.go - Application port for user presets.
##
*/

package ports

import (
	"context"

	"overdrive/services/user-data-service/src/core/domain"
)

// PresetUseCase exposes the /presets application logic. userID always comes from the verified
// access token, never from the request path, query or body. Errors are domain sentinels:
// ErrInvalidPreset (400), ErrPresetNotFound (404), ErrPresetLimitReached (409).
type PresetUseCase interface {
	List(ctx context.Context, userID string) ([]domain.Preset, error)
	Get(ctx context.Context, userID string, presetID string) (*domain.Preset, error)
	Create(ctx context.Context, userID string, input domain.NewPreset) (*domain.Preset, error)
	Update(ctx context.Context, userID string, presetID string, changes domain.PresetChanges) (*domain.Preset, error)
	Delete(ctx context.Context, userID string, presetID string) error
}
