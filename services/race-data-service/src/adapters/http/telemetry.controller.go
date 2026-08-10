/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## telemetry.controller.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"net/http"

	"overdrive/services/race-data-service/src/core/ports"
	"overdrive/shared/apierror"
)

type TelemetryController struct {
	usecase ports.TelemetryUseCase
}

// NewTelemetryController builds and returns a telemetry controller with its required dependencies.
func NewTelemetryController(usecase ports.TelemetryUseCase) *TelemetryController {
	return &TelemetryController{usecase: usecase}
}

// GetSpeed returns every speed sample for the requested session/driver.
func (c *TelemetryController) GetSpeed(w http.ResponseWriter, r *http.Request) {
	sessionID, driverNumber, lapNumber, ok := c.parseCommon(w, r)
	if !ok {
		return
	}
	items, err := c.usecase.GetSpeed(r.Context(), sessionID, driverNumber, lapNumber)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load speed telemetry", err))
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetEngine returns every engine sample for the requested session/driver.
func (c *TelemetryController) GetEngine(w http.ResponseWriter, r *http.Request) {
	sessionID, driverNumber, lapNumber, ok := c.parseCommon(w, r)
	if !ok {
		return
	}
	items, err := c.usecase.GetEngine(r.Context(), sessionID, driverNumber, lapNumber)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load engine telemetry", err))
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetLocation returns spatial telemetry for the requested session/driver.
func (c *TelemetryController) GetLocation(w http.ResponseWriter, r *http.Request) {
	sessionID, driverNumber, lapNumber, ok := c.parseCommon(w, r)
	if !ok {
		return
	}
	items, err := c.usecase.GetLocation(r.Context(), sessionID, driverNumber, lapNumber)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load location telemetry", err))
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetIntervals returns interval telemetry for the requested session/driver.
func (c *TelemetryController) GetIntervals(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	driverNumber, ok := parsePathPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	if !c.requireSession(w, r, sessionID) {
		return
	}
	payload, err := c.usecase.GetIntervals(r.Context(), sessionID, driverNumber)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load interval telemetry", err))
		return
	}
	if payload == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("TELEMETRY", "telemetry not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// parseCommon parses the shared sessionId/driverNumber/lapNumber parameters used
// by the speed, engine, and location telemetry endpoints, and validates the session.
func (c *TelemetryController) parseCommon(w http.ResponseWriter, r *http.Request) (string, int, *int, bool) {
	sessionID := r.PathValue("sessionId")
	driverNumber, ok := parsePathPositiveInt(w, r, "driverNumber")
	if !ok {
		return "", 0, nil, false
	}
	if !c.requireSession(w, r, sessionID) {
		return "", 0, nil, false
	}
	lapNumber, ok := parseOptionalPositiveInt(w, r, "lapNumber")
	if !ok {
		return "", 0, nil, false
	}
	return sessionID, driverNumber, lapNumber, true
}

// requireSession validates the sessionId path parameter against championship-service
// and writes a consistent 404 response if it is unknown.
func (c *TelemetryController) requireSession(w http.ResponseWriter, r *http.Request, sessionID string) bool {
	exists, err := c.usecase.SessionExists(r.Context(), sessionID)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.UpstreamUnavailable("failed to verify session with championship-service", err))
		return false
	}
	if !exists {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return false
	}
	return true
}
