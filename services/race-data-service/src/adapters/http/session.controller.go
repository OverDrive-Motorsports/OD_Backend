/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## session.controller.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"net/http"
	"strconv"

	"overdrive/services/race-data-service/src/core/ports"
)

type SessionController struct {
	usecase ports.SessionQueryUseCase
}

// NewSessionController builds and returns a session controller with its required dependencies.
func NewSessionController(usecase ports.SessionQueryUseCase) *SessionController {
	return &SessionController{usecase: usecase}
}

// GetSession returns the requested session payload for the supplied identifiers.
func (c *SessionController) GetSession(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSession(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// ListChampionships returns a collection of championships for the requested context.
func (c *SessionController) ListChampionships(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.ListChampionships(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// ListChampionshipEvents returns a collection of championship events for the requested context.
func (c *SessionController) ListChampionshipEvents(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.ListChampionshipEvents(r.Context(), r.PathValue("code"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "championship not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetEvent returns the requested event payload for the supplied identifiers.
func (c *SessionController) GetEvent(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetEvent(r.Context(), r.PathValue("eventId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// ListEventSessions returns a collection of event sessions for the requested context.
func (c *SessionController) ListEventSessions(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.ListEventSessions(r.Context(), r.PathValue("eventId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetSessionMetadata returns the requested session metadata payload for the supplied identifiers.
func (c *SessionController) GetSessionMetadata(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionMetadata(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// ListSessionDrivers returns a collection of session drivers for the requested context.
func (c *SessionController) ListSessionDrivers(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.ListSessionDrivers(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// ListSessionTeams returns a collection of session teams for the requested context.
func (c *SessionController) ListSessionTeams(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.ListSessionTeams(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetSessionDataset returns the requested session dataset payload for the supplied identifiers.
func (c *SessionController) GetSessionDataset(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionDataset(r.Context(), r.PathValue("sessionId"), r.PathValue("dataset"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetSessionRaceStandings returns the requested session race standings payload for the supplied identifiers.
func (c *SessionController) GetSessionRaceStandings(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionRaceStandings(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetSessionBroadcast returns the requested session broadcast payload for the supplied identifiers.
func (c *SessionController) GetSessionBroadcast(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionBroadcast(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetSessionWeather returns the requested session weather payload for the supplied identifiers.
func (c *SessionController) GetSessionWeather(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionDataset(r.Context(), r.PathValue("sessionId"), "weather")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetSessionFacts returns the requested session facts payload for the supplied identifiers.
func (c *SessionController) GetSessionFacts(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionFacts(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetDriverProfile returns the requested driver profile payload for the supplied identifiers.
func (c *SessionController) GetDriverProfile(w http.ResponseWriter, r *http.Request) {
	driverNumber, err := strconv.Atoi(r.PathValue("driverNumber"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid driver number"})
		return
	}
	payload, err := c.usecase.GetDriverProfile(r.Context(), r.PathValue("sessionId"), driverNumber)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "driver not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetDriverBroadcast returns the requested driver broadcast payload for the supplied identifiers.
func (c *SessionController) GetDriverBroadcast(w http.ResponseWriter, r *http.Request) {
	driverNumber, err := strconv.Atoi(r.PathValue("driverNumber"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid driver number"})
		return
	}
	payload, err := c.usecase.GetDriverBroadcast(r.Context(), r.PathValue("sessionId"), driverNumber)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetDriverDataset returns the requested driver dataset payload for the supplied identifiers.
func (c *SessionController) GetDriverDataset(w http.ResponseWriter, r *http.Request) {
	driverNumber, err := strconv.Atoi(r.PathValue("driverNumber"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid driver number"})
		return
	}
	payload, err := c.usecase.GetDriverDataset(r.Context(), r.PathValue("sessionId"), driverNumber, r.PathValue("dataset"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetDriverLapLocation returns the requested driver lap location payload for the supplied identifiers.
func (c *SessionController) GetDriverLapLocation(w http.ResponseWriter, r *http.Request) {
	driverNumber, err := strconv.Atoi(r.PathValue("driverNumber"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid driver number"})
		return
	}
	lapNumber, err := strconv.Atoi(r.PathValue("lapNumber"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid lap number"})
		return
	}
	payload, err := c.usecase.GetDriverLapLocation(r.Context(), r.PathValue("sessionId"), driverNumber, lapNumber)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}
