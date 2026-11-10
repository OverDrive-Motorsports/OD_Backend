/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset.controller.go - HTTP handlers for the caller's presets.
##
*/

package httpadapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"overdrive/services/user-data-service/src/core/domain"
	"overdrive/services/user-data-service/src/core/ports"
	"overdrive/shared/apierror"
)

type PresetController struct {
	usecase ports.PresetUseCase
}

// NewPresetController builds and returns a preset controller with its required dependencies.
func NewPresetController(usecase ports.PresetUseCase) *PresetController {
	return &PresetController{usecase: usecase}
}

// createPresetBody is the POST body. data stays raw: it is the client's layout document.
type createPresetBody struct {
	Name      string          `json:"name"`
	IsDefault bool            `json:"isDefault"`
	Data      json.RawMessage `json:"data"`
}

// updatePresetBody is the PATCH body; an absent field is left unchanged.
type updatePresetBody struct {
	Name      *string         `json:"name"`
	IsDefault *bool           `json:"isDefault"`
	Data      json.RawMessage `json:"data"`
}

// List answers the caller's presets, newest first, as a bare array.
func (c *PresetController) List(w http.ResponseWriter, r *http.Request) {
	presets, err := c.usecase.List(r.Context(), userIDFrom(r))
	if err != nil {
		writePresetError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, presets)
}

// Get answers one of the caller's presets.
func (c *PresetController) Get(w http.ResponseWriter, r *http.Request) {
	preset, err := c.usecase.Get(r.Context(), userIDFrom(r), r.PathValue("presetId"))
	if err != nil {
		writePresetError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, preset)
}

// Create stores a new preset for the caller and answers 201 with it.
func (c *PresetController) Create(w http.ResponseWriter, r *http.Request) {
	var body createPresetBody
	if err := decodeBody(r, &body); err != nil {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid preset body", err))
		return
	}
	preset, err := c.usecase.Create(r.Context(), userIDFrom(r), domain.NewPreset{Name: body.Name, IsDefault: body.IsDefault, Data: body.Data})
	if err != nil {
		writePresetError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, preset)
}

// Update applies a partial update to one of the caller's presets.
func (c *PresetController) Update(w http.ResponseWriter, r *http.Request) {
	var body updatePresetBody
	if err := decodeBody(r, &body); err != nil {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid preset body", err))
		return
	}
	preset, err := c.usecase.Update(r.Context(), userIDFrom(r), r.PathValue("presetId"), domain.PresetChanges{Name: body.Name, IsDefault: body.IsDefault, Data: body.Data})
	if err != nil {
		writePresetError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, preset)
}

// Delete removes one of the caller's presets and answers 204.
func (c *PresetController) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.usecase.Delete(r.Context(), userIDFrom(r), r.PathValue("presetId")); err != nil {
		writePresetError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeBody decodes exactly one JSON object, rejecting unknown fields and trailing data. A
// `"data": null` is treated as absent.
func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("unexpected data after the JSON body")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("unexpected data after the JSON body")
	}
	switch body := target.(type) {
	case *createPresetBody:
		body.Data = nullToNil(body.Data)
	case *updatePresetBody:
		body.Data = nullToNil(body.Data)
	}
	return nil
}

// nullToNil turns a JSON `null` into an absent value.
func nullToNil(raw json.RawMessage) json.RawMessage {
	if string(raw) == "null" {
		return nil
	}
	return raw
}

// writePresetError maps domain sentinels to the shared error contract; anything else is a 500
// whose detail stays in the server log.
func writePresetError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPreset):
		apierror.Write(w, r.URL.Path, apierror.Validation(err.Error(), err))
	case errors.Is(err, domain.ErrPresetNotFound):
		apierror.Write(w, r.URL.Path, apierror.NotFound("PRESET", "preset not found", nil))
	case errors.Is(err, domain.ErrPresetLimitReached):
		apierror.Write(w, r.URL.Path, apierror.Conflict("PRESET", fmt.Sprintf("a user can own at most %d presets", domain.MaxPresetsPerUser), err))
	default:
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to process preset", err))
	}
}
