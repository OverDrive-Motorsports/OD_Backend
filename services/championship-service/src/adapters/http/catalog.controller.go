/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## catalog.controller.go - Package httpadapter source file for services/championship-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"overdrive/services/championship-service/src/core/domain"
	"overdrive/services/championship-service/src/core/ports"
	"overdrive/shared/apierror"
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
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load championships", err))
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
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load championship events", err))
		return
	}
	if items == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("CHAMPIONSHIP", "championship not found", nil))
		return
	}
	if seasonRaw := strings.TrimSpace(r.URL.Query().Get("season")); seasonRaw != "" {
		season, err := strconv.Atoi(seasonRaw)
		if err != nil {
			apierror.Write(w, r.URL.Path, apierror.Validation("invalid season parameter", err))
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
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load event", err))
		return
	}
	if event == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("EVENT", "event not found", nil))
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
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load event sessions", err))
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
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load session", err))
		return
	}
	if session == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
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
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load session drivers", err))
		return
	}
	if items == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// ListSessionTeams returns a collection of session teams for the requested context.
func (c *CatalogController) ListSessionTeams(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	items, err := c.usecase.ListSessionTeams(r.Context(), sessionID)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load session teams", err))
		return
	}
	if items == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetSessionDataset returns the requested session dataset payload for the supplied identifiers.
func (c *CatalogController) GetSessionDataset(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionDataset(r.Context(), r.PathValue("sessionId"), r.PathValue("dataset"))
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyDatasetErr(err))
		return
	}
	if payload.SessionID == "" {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
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
		apierror.Write(w, r.URL.Path, classifyDatasetErr(err))
		return
	}
	if payload.SessionID == "" {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
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
			apierror.Write(w, r.URL.Path, apierror.Validation("invalid driverNumber parameter", err))
			return
		}
		driverNumber = &value
	}
	items, err := c.usecase.GetSessionStandings(r.Context(), sessionID, driverNumber)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load session standings", err))
		return
	}
	if items == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetDriverProfile returns the global (session-independent) driver profile for the
// supplied driver number, optionally scoped by the "championshipCode" query parameter.
func (c *CatalogController) GetDriverProfile(w http.ResponseWriter, r *http.Request) {
	driverNumber, err := strconv.Atoi(r.PathValue("driverNumber"))
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid driver number", err))
		return
	}
	championshipCode := strings.TrimSpace(r.URL.Query().Get("championshipCode"))
	profile, err := c.usecase.GetDriverProfile(r.Context(), driverNumber, championshipCode)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load driver profile", err))
		return
	}
	if profile == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("DRIVER", "driver not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// GetSessionBroadcast returns the requested session broadcast payload for the supplied identifiers.
func (c *CatalogController) GetSessionBroadcast(w http.ResponseWriter, r *http.Request) {
	session, err := c.usecase.GetSession(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load session", err))
		return
	}
	if session == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId":    session.ID,
		"broadcastUrl": session.BroadcastURL,
	})
}

// classifyDatasetErr maps a catalog repository error into the matching apierror. An unknown
// dataset name is a client validation error (400) - aligned with the gateway's own dataset
// allow-list validation in race_parameter_middleware.go, which already rejects an unrecognized
// dataset with 400 rather than 404 at the edge. Anything else is an unexpected repository
// failure (500).
func classifyDatasetErr(err error) *apierror.Error {
	if errors.Is(err, domain.ErrUnknownDataset) {
		return apierror.Validation("invalid dataset", err)
	}
	return apierror.Internal("failed to load session dataset", err)
}
