/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset.entity.go - User layout preset and its validation rules.
##
*/

package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits enforced on presets.
const (
	MaxPresetsPerUser   = 20
	MaxPresetNameLength = 50
	// MaxPresetDataBytes caps the client layout document stored in Preset.Data.
	MaxPresetDataBytes = 256 * 1024
)

// Preset is one saved AR/VR layout of a user. Data is the client's layout document (widgets,
// positions, sizes, settings...) stored as-is: the backend only guarantees it is a JSON object
// within MaxPresetDataBytes and never interprets it. Postgres stores it as jsonb, so key order
// and whitespace are normalised on the way back, the content is unchanged.
type Preset struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	IsDefault bool            `json:"isDefault"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// NewPreset is the input of a preset creation.
type NewPreset struct {
	Name      string
	IsDefault bool
	Data      json.RawMessage
}

// PresetChanges is a partial update: a nil field is left unchanged.
type PresetChanges struct {
	Name      *string
	IsDefault *bool
	Data      json.RawMessage
}

// IsEmpty reports whether the update changes nothing.
func (c PresetChanges) IsEmpty() bool {
	return c.Name == nil && c.IsDefault == nil && c.Data == nil
}

// Validate checks a creation payload; the returned error wraps ErrInvalidPreset.
func (p NewPreset) Validate() error {
	if err := ValidatePresetName(p.Name); err != nil {
		return err
	}
	return ValidatePresetData(p.Data)
}

// Validate checks a partial update; at least one field must be set.
func (c PresetChanges) Validate() error {
	if c.IsEmpty() {
		return fmt.Errorf("%w: at least one of name, isDefault or data is required", ErrInvalidPreset)
	}
	if c.Name != nil {
		if err := ValidatePresetName(*c.Name); err != nil {
			return err
		}
	}
	if c.Data != nil {
		return ValidatePresetData(c.Data)
	}
	return nil
}

// ValidatePresetName requires a non-blank name of at most MaxPresetNameLength characters
// (counted in runes, matching the varchar(50) column).
func ValidatePresetName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidPreset)
	}
	if utf8.RuneCountInString(name) > MaxPresetNameLength {
		return fmt.Errorf("%w: name must be at most %d characters", ErrInvalidPreset, MaxPresetNameLength)
	}
	return nil
}

// ValidatePresetData requires a well-formed JSON object of at most MaxPresetDataBytes.
func ValidatePresetData(data json.RawMessage) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("%w: data is required", ErrInvalidPreset)
	}
	if len(trimmed) > MaxPresetDataBytes {
		return fmt.Errorf("%w: data must be at most %d bytes", ErrInvalidPreset, MaxPresetDataBytes)
	}
	if trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("%w: data must be a JSON object", ErrInvalidPreset)
	}
	return nil
}
