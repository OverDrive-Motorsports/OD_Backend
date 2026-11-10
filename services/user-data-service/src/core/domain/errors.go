/**
##
## OverDrive 2026
## All Technical rights reserved
##
## errors.go - Domain sentinel errors for user-data-service.
##
*/

package domain

import "errors"

var (
	// ErrInvalidPreset is returned when a preset payload breaks a validation rule; the wrapped
	// message names the field and is safe to show to the client (400).
	ErrInvalidPreset = errors.New("invalid preset")
	// ErrPresetNotFound is returned when the preset does not exist OR belongs to another user:
	// both answer 404 so a caller cannot probe other users' preset IDs.
	ErrPresetNotFound = errors.New("preset not found")
	// ErrPresetLimitReached is returned when the user already owns MaxPresetsPerUser presets (409).
	ErrPresetLimitReached = errors.New("preset limit reached")
)
