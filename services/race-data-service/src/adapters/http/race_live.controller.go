/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_live.controller.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"net/http"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

// raceControlLongPollTimeout bounds how long POST /race/control blocks before
// returning an empty array, forcing the client to re-POST (long-polling contract).
const raceControlLongPollTimeout = 30 * time.Second

type RaceLiveController struct {
	usecase ports.RaceLiveUseCase
}

// NewRaceLiveController builds and returns a race live controller with its required dependencies.
func NewRaceLiveController(usecase ports.RaceLiveUseCase) *RaceLiveController {
	return &RaceLiveController{usecase: usecase}
}

// GetPosition returns live position data for the requested session.
func (c *RaceLiveController) GetPosition(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if !c.requireSession(w, r, sessionID) {
		return
	}
	driverNumber, ok := parseOptionalPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	lapNumber, ok := parseOptionalPositiveInt(w, r, "lapNumber")
	if !ok {
		return
	}
	payload, err := c.usecase.GetPosition(r.Context(), sessionID, driverNumber, lapNumber)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "position data not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetLaps returns lap timing data for the requested session.
func (c *RaceLiveController) GetLaps(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if !c.requireSession(w, r, sessionID) {
		return
	}
	driverNumber, ok := parseOptionalPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	lapNumber, ok := parseOptionalPositiveInt(w, r, "lapNumber")
	if !ok {
		return
	}
	payload, err := c.usecase.GetLaps(r.Context(), sessionID, driverNumber, lapNumber)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "lap data not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetStints returns tyre stint data for the requested session.
func (c *RaceLiveController) GetStints(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if !c.requireSession(w, r, sessionID) {
		return
	}
	driverNumber, ok := parseOptionalPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	items, err := c.usecase.GetStints(r.Context(), sessionID, driverNumber)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetPitStops returns pit stop data for the requested session.
func (c *RaceLiveController) GetPitStops(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if !c.requireSession(w, r, sessionID) {
		return
	}
	driverNumber, ok := parseOptionalPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	items, err := c.usecase.GetPitStops(r.Context(), sessionID, driverNumber)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetWeather returns weather samples for the requested session.
func (c *RaceLiveController) GetWeather(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if !c.requireSession(w, r, sessionID) {
		return
	}
	items, err := c.usecase.GetWeather(r.Context(), sessionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetRadio returns team radio messages for the requested session.
func (c *RaceLiveController) GetRadio(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if !c.requireSession(w, r, sessionID) {
		return
	}
	driverNumber, ok := parseOptionalPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	items, err := c.usecase.GetRadio(r.Context(), sessionID, driverNumber)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// PostRaceControl implements the POST /sessions/{sessionId}/race/control long-poll
// contract: it blocks until a new event batch is available (or a bounded timeout
// elapses), then closes the response. The front-end must re-POST to keep receiving
// events. See race_control_broadcaster.go for the single-instance caveat.
func (c *RaceLiveController) PostRaceControl(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if !c.requireSession(w, r, sessionID) {
		return
	}
	events, err := c.usecase.WaitForRaceControl(r.Context(), sessionID, raceControlLongPollTimeout)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if events == nil {
		events = []domain.RaceControlEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

// requireSession validates the sessionId path parameter against championship-service
// and writes a consistent 404 response if it is unknown.
func (c *RaceLiveController) requireSession(w http.ResponseWriter, r *http.Request, sessionID string) bool {
	exists, err := c.usecase.SessionExists(r.Context(), sessionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return false
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return false
	}
	return true
}
