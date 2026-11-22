/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset_entity_test.go - Unit tests for the preset validation rules.
##
*/

package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestValidatePresetName proves a name must be non-blank and at most 50 characters, counted
// in runes so accented names are not cut short.
func TestValidatePresetName(t *testing.T) {
	for _, name := range []string{"Layout", strings.Repeat("é", MaxPresetNameLength)} {
		if err := ValidatePresetName(name); err != nil {
			t.Fatalf("%q: unexpected error %v", name, err)
		}
	}
	for _, name := range []string{"", "   ", strings.Repeat("a", MaxPresetNameLength+1)} {
		if err := ValidatePresetName(name); !errors.Is(err, ErrInvalidPreset) {
			t.Fatalf("%q: expected ErrInvalidPreset, got %v", name, err)
		}
	}
}

// TestValidatePresetData proves data must be a well-formed JSON object within the size cap;
// arrays, scalars, null, malformed JSON and an empty value are rejected.
func TestValidatePresetData(t *testing.T) {
	for _, data := range []string{`{}`, ` {"widgets":[{"type":"video","x":1.5}]} `} {
		if err := ValidatePresetData(json.RawMessage(data)); err != nil {
			t.Fatalf("%s: unexpected error %v", data, err)
		}
	}
	tooBig := `{"blob":"` + strings.Repeat("a", MaxPresetDataBytes) + `"}`
	for name, data := range map[string]string{
		"empty": "", "blank": "  ", "array": `[]`, "string": `"x"`, "number": `1`,
		"null": `null`, "malformed": `{"a":`, "too big": tooBig,
	} {
		if err := ValidatePresetData(json.RawMessage(data)); !errors.Is(err, ErrInvalidPreset) {
			t.Fatalf("%s: expected ErrInvalidPreset, got %v", name, err)
		}
	}
}

// TestNewPresetAndChanges_Validate proves creation needs a valid name and data, and a partial
// update needs at least one field, each set field being validated.
func TestNewPresetAndChanges_Validate(t *testing.T) {
	if err := (NewPreset{Name: "A", Data: json.RawMessage(`{}`)}).Validate(); err != nil {
		t.Fatalf("valid creation: %v", err)
	}
	if err := (NewPreset{Name: "A"}).Validate(); !errors.Is(err, ErrInvalidPreset) {
		t.Fatalf("creation without data: expected ErrInvalidPreset, got %v", err)
	}

	empty := PresetChanges{}
	if !empty.IsEmpty() || !errors.Is(empty.Validate(), ErrInvalidPreset) {
		t.Fatal("an empty update must be rejected")
	}
	flag := true
	if err := (PresetChanges{IsDefault: &flag}).Validate(); err != nil {
		t.Fatalf("default-only update: %v", err)
	}
	blank := " "
	if err := (PresetChanges{Name: &blank}).Validate(); !errors.Is(err, ErrInvalidPreset) {
		t.Fatalf("blank name update: expected ErrInvalidPreset, got %v", err)
	}
	if err := (PresetChanges{Data: json.RawMessage(`[1]`)}).Validate(); !errors.Is(err, ErrInvalidPreset) {
		t.Fatalf("non-object data update: expected ErrInvalidPreset, got %v", err)
	}
}
