/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_handler.go - Race HTTP handlers for fetch, cache status, and payload delivery endpoints.
##
*/

package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"overdrive/internal/service"
	"overdrive/internal/usecase"
)

type RaceDefaults struct {
	Year         int
	CountryName  string
	MeetingName  string
	DriverNumber int
}

type RaceHandler struct {
	raceService  *service.RaceService
	defaults     RaceDefaults
	fetchTimeout time.Duration
}

// NewRaceHandler creates an HTTP handler with race defaults and fetch timeout settings.
func NewRaceHandler(raceService *service.RaceService, defaults RaceDefaults, fetchTimeout time.Duration) *RaceHandler {
	return &RaceHandler{
		raceService:  raceService,
		defaults:     defaults,
		fetchTimeout: fetchTimeout,
	}
}

// HandleRoot returns basic service metadata and the list of available routes.
func (h *RaceHandler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service": "overdrive-backend",
		"version": "v1",
		"routes": []string{
			"GET /health",
			"GET /getrace",
			"POST /sendrace",
			"GET /api/v1/race/getrace",
			"POST /api/v1/race/sendrace",
			"GET /api/v1/race/cache",
		},
	})
}

// HandleHealth reports service liveness information.
func (h *RaceHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

// HandleGetRace fetches race data from the provider and stores it in cache.
func (h *RaceHandler) HandleGetRace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	input, err := h.parseInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.fetchTimeout)
	defer cancel()

	archive, err := h.raceService.FetchAndCache(ctx, input)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch race data: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "cached",
		"metadata":     archive.Metadata,
		"counts":       archive.Counts,
		"fetch_errors": archive.FetchErrors,
	})
}

// HandleSendRace returns the last cached race payload to the caller.
func (h *RaceHandler) HandleSendRace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, cachedAt, found, err := h.raceService.GetCached(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read cache: %v", err))
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "no race data cached yet, call GET /getrace first")
		return
	}

	w.Header().Set("X-Cached-At", cachedAt.Format(time.RFC3339))
	writeJSON(w, http.StatusOK, archive)
}

// HandleCacheStatus reports whether a race payload is currently cached.
func (h *RaceHandler) HandleCacheStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, cachedAt, found, err := h.raceService.GetCached(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read cache: %v", err))
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{
			"cached": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"cached":    true,
		"cached_at": cachedAt,
		"metadata":  archive.Metadata,
	})
}

// parseInput merges query parameters with handler defaults into a RaceBuildInput.
func (h *RaceHandler) parseInput(r *http.Request) (usecase.RaceBuildInput, error) {
	q := r.URL.Query()
	input := usecase.RaceBuildInput{
		Year:         h.defaults.Year,
		CountryName:  h.defaults.CountryName,
		MeetingName:  h.defaults.MeetingName,
		DriverNumber: h.defaults.DriverNumber,
	}

	if v := strings.TrimSpace(q.Get("year")); v != "" {
		year, err := strconv.Atoi(v)
		if err != nil || year <= 0 {
			return usecase.RaceBuildInput{}, fmt.Errorf("invalid year: %q", v)
		}
		input.Year = year
	}

	if v := strings.TrimSpace(q.Get("country")); v != "" {
		input.CountryName = v
	}
	if v := strings.TrimSpace(q.Get("meeting")); v != "" {
		input.MeetingName = v
	}

	if v := strings.TrimSpace(q.Get("driver_number")); v != "" {
		driverNumber, err := strconv.Atoi(v)
		if err != nil || driverNumber < 0 {
			return usecase.RaceBuildInput{}, fmt.Errorf("invalid driver_number: %q", v)
		}
		input.DriverNumber = driverNumber
	}

	return input, nil
}
