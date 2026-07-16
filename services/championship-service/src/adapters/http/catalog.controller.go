/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## catalog.controller.go - Package httpadapter source file for services/championship-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"net/http"
	"strconv"
	"strings"

	"overdrive/services/championship-service/src/core/ports"
)

type CatalogController struct {
	usecase ports.CatalogQueryUseCase
}

// NewCatalogController builds and returns a catalog controller with its required dependencies.
func NewCatalogController(usecase ports.CatalogQueryUseCase) *CatalogController {
	return &CatalogController{usecase: usecase}
}

// ListChampionships returns a collection of championships for the requested context.
func (c *CatalogController) ListChampionships(w http.ResponseWriter, r *http.Request) {
	items, err := c.usecase.ListChampionships(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// ListChampionshipEvents returns a collection of championship events for the requested context,
// optionally filtered by the "season" query parameter.
func (c *CatalogController) ListChampionshipEvents(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	items, err := c.usecase.ListEventsByChampionship(r.Context(), code)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "championship not found"})
		return
	}
	if seasonRaw := strings.TrimSpace(r.URL.Query().Get("season")); seasonRaw != "" {
		season, err := strconv.Atoi(seasonRaw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid season parameter"})
			return
		}
		filtered := items[:0:0]
		for _, item := range items {
			if item.SeasonYear == season {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, items)
}

// GetEvent returns the requested event payload for the supplied identifiers.
func (c *CatalogController) GetEvent(w http.ResponseWriter, r *http.Request) {
	event, err := c.usecase.GetEvent(r.Context(), r.PathValue("eventId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if event == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}
	writeJSON(w, http.StatusOK, event)
}

// ListEventSessions returns a collection of event sessions for the requested context,
// optionally filtered by the "type" query parameter (practice|qualifying|race|sprint).
func (c *CatalogController) ListEventSessions(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("eventId")
	items, err := c.usecase.ListSessionsByEvent(r.Context(), eventID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if sessionType := strings.TrimSpace(r.URL.Query().Get("type")); sessionType != "" {
		if strings.EqualFold(sessionType, "qualifying") {
			sessionType = "quali"
		}
		filtered := items[:0:0]
		for _, item := range items {
			if strings.EqualFold(item.Type, sessionType) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, items)
}

// GetSession returns the requested session payload for the supplied identifiers.
func (c *CatalogController) GetSession(w http.ResponseWriter, r *http.Request) {
	session, err := c.usecase.GetSession(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if session == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// ListSessionDrivers returns a collection of session drivers for the requested context,
// optionally filtered by the "teamId" query parameter.
func (c *CatalogController) ListSessionDrivers(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	teamID := strings.TrimSpace(r.URL.Query().Get("teamId"))
	items, err := c.usecase.ListSessionDrivers(r.Context(), sessionID, teamID)
	if err != nil {
		writeJSON(w, statusFromErr(err), map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// ListSessionTeams returns a collection of session teams for the requested context.
func (c *CatalogController) ListSessionTeams(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	items, err := c.usecase.ListSessionTeams(r.Context(), sessionID)
	if err != nil {
		writeJSON(w, statusFromErr(err), map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetSessionDataset returns the requested session dataset payload for the supplied identifiers.
func (c *CatalogController) GetSessionDataset(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionDataset(r.Context(), r.PathValue("sessionId"), r.PathValue("dataset"))
	if err != nil {
		writeJSON(w, statusFromErr(err), map[string]string{"error": err.Error()})
		return
	}
	if payload.SessionID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetSessionRaceStandings returns the requested session race standings payload for the supplied identifiers.
// Deprecated: superseded by GetSessionStandings (GET /sessions/{sessionId}/standings), kept for
// backward compatibility with existing internal consumers of /standings/race.
func (c *CatalogController) GetSessionRaceStandings(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionDataset(r.Context(), r.PathValue("sessionId"), "session_result")
	if err != nil {
		writeJSON(w, statusFromErr(err), map[string]string{"error": err.Error()})
		return
	}
	if payload.SessionID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// GetSessionStandings returns the generic session standings (race result), optionally
// isolating a single driver via the "driverNumber" query parameter.
func (c *CatalogController) GetSessionStandings(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	var driverNumber *int
	if raw := strings.TrimSpace(r.URL.Query().Get("driverNumber")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid driverNumber parameter"})
			return
		}
		driverNumber = &value
	}
	items, err := c.usecase.GetSessionStandings(r.Context(), sessionID, driverNumber)
	if err != nil {
		writeJSON(w, statusFromErr(err), map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetDriverProfile returns the global (session-independent) driver profile for the
// supplied driver number, optionally scoped by the "championshipCode" query parameter.
func (c *CatalogController) GetDriverProfile(w http.ResponseWriter, r *http.Request) {
	driverNumber, err := strconv.Atoi(r.PathValue("driverNumber"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid driver number"})
		return
	}
	championshipCode := strings.TrimSpace(r.URL.Query().Get("championshipCode"))
	profile, err := c.usecase.GetDriverProfile(r.Context(), driverNumber, championshipCode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if profile == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "driver not found"})
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// GetSessionBroadcast returns the requested session broadcast payload for the supplied identifiers.
func (c *CatalogController) GetSessionBroadcast(w http.ResponseWriter, r *http.Request) {
	session, err := c.usecase.GetSession(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if session == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId":    session.ID,
		"broadcastUrl": session.BroadcastURL,
	})
}

// statusFromErr maps domain or repository errors to the matching HTTP status code.
func statusFromErr(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if strings.Contains(err.Error(), "unknown dataset") {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
