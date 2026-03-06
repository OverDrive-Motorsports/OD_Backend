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
	"sort"
	"strconv"
	"strings"
	"time"

	"overdrive/internal/domain"
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

type raceStandingSnapshot struct {
	DriverNumber int
	Position     int
	Date         time.Time
	Row          map[string]any
}

var driverRaceDatasetKeys = []string{
	"laps",
	"car_data",
	"location",
	"position",
	"intervals",
	"stints",
	"pit",
	"team_radio",
	"overtakes",
	"session_result",
	"starting_grid",
	"championship_drivers",
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
			"GET /api/v1/race/championship/drivers",
			"GET /api/v1/race/championship/constructors",
			"GET /api/v1/race/weather",
			"GET /api/v1/race/facts",
			"GET /api/v1/race/driver?driver_number=63",
			"GET /api/v1/race/standings/race",
			"GET /api/v1/race/video-url",
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

// HandleSendDriverChampionship returns the cached drivers championship standings dataset.
func (h *RaceHandler) HandleSendDriverChampionship(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getCachedArchive(w, r)
	if !ok {
		return
	}

	data := archive.Datasets["championship_drivers"]
	writeJSON(w, http.StatusOK, map[string]any{
		"dataset":  "championship_drivers",
		"metadata": archive.Metadata,
		"count":    len(data),
		"data":     data,
	})
}

// HandleSendConstructorChampionship returns the cached constructors championship standings dataset.
func (h *RaceHandler) HandleSendConstructorChampionship(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getCachedArchive(w, r)
	if !ok {
		return
	}

	data := archive.Datasets["championship_teams"]
	writeJSON(w, http.StatusOK, map[string]any{
		"dataset":  "championship_teams",
		"metadata": archive.Metadata,
		"count":    len(data),
		"data":     data,
	})
}

// HandleSendWeather returns the cached race weather dataset.
func (h *RaceHandler) HandleSendWeather(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getCachedArchive(w, r)
	if !ok {
		return
	}

	data := archive.Datasets["weather"]
	writeJSON(w, http.StatusOK, map[string]any{
		"dataset":  "weather",
		"metadata": archive.Metadata,
		"count":    len(data),
		"data":     data,
	})
}

// HandleSendRaceFacts returns race control events such as yellow flags and incidents.
func (h *RaceHandler) HandleSendRaceFacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getCachedArchive(w, r)
	if !ok {
		return
	}

	data := archive.Datasets["race_control"]
	writeJSON(w, http.StatusOK, map[string]any{
		"dataset":  "race_control",
		"metadata": archive.Metadata,
		"count":    len(data),
		"data":     data,
	})
}

// HandleSendDriverRace returns all cached race datasets filtered for one driver.
func (h *RaceHandler) HandleSendDriverRace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getCachedArchive(w, r)
	if !ok {
		return
	}

	driverNumber, err := h.parseDriverNumberForRace(r, archive)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	driverInfo := filterRowsByDriver(archive.Datasets["drivers"], driverNumber)
	if len(driverInfo) == 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("driver_number=%d not found in cached race", driverNumber))
		return
	}

	data := make(map[string][]map[string]any, len(driverRaceDatasetKeys))
	counts := make(map[string]int, len(driverRaceDatasetKeys))

	for _, key := range driverRaceDatasetKeys {
		rows := filterRowsByDriver(archive.Datasets[key], driverNumber)
		data[key] = rows
		counts[key] = len(rows)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"driver_number": driverNumber,
		"metadata":      archive.Metadata,
		"driver":        driverInfo[0],
		"counts":        counts,
		"data":          data,
	})
}

// HandleSendRaceStandings returns race standings snapshots based on in-race position updates.
func (h *RaceHandler) HandleSendRaceStandings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getCachedArchive(w, r)
	if !ok {
		return
	}

	if archive.Metadata.DriverNumber > 0 {
		writeError(
			w,
			http.StatusConflict,
			"race standings require a full-race cache; call GET /api/v1/race/getrace with driver_number=0 first",
		)
		return
	}

	at, hasAt, err := parseSnapshotTime(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows := archive.Datasets["position"]
	if len(rows) == 0 {
		writeError(w, http.StatusNotFound, "position dataset is empty in cache")
		return
	}

	snapshotAt, standings := buildRaceStandings(rows, at, hasAt)
	if len(standings) == 0 {
		if hasAt {
			writeError(w, http.StatusNotFound, fmt.Sprintf("no position rows found at or before %s", at.Format(time.RFC3339)))
			return
		}
		writeError(w, http.StatusNotFound, "no valid position rows found in cache")
		return
	}

	driversByNumber := indexDriversByNumber(archive.Datasets["drivers"])
	payload := make([]map[string]any, 0, len(standings))

	for _, item := range standings {
		row := cloneRow(item.Row)
		if driver, found := driversByNumber[item.DriverNumber]; found {
			row["driver"] = driver
		}
		payload = append(payload, row)
	}

	out := map[string]any{
		"dataset":     "position",
		"metadata":    archive.Metadata,
		"snapshot_at": snapshotAt,
		"count":       len(payload),
		"data":        payload,
	}
	if hasAt {
		out["requested_at"] = at
	}

	writeJSON(w, http.StatusOK, out)
}

// HandleSendVideoURL returns a placeholder broadcast URL for the selected event.
func (h *RaceHandler) HandleSendVideoURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getCachedArchive(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"provider": "youtube",
		"title":    "OverDrive Race Broadcast Placeholder",
		"url":      "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"metadata": archive.Metadata,
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

// getCachedArchive loads the latest race archive and writes an HTTP error when unavailable.
func (h *RaceHandler) getCachedArchive(w http.ResponseWriter, r *http.Request) (domain.RaceArchive, time.Time, bool) {
	archive, cachedAt, found, err := h.raceService.GetCached(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read cache: %v", err))
		return domain.RaceArchive{}, time.Time{}, false
	}
	if !found {
		writeError(w, http.StatusNotFound, "no race data cached yet, call GET /getrace first")
		return domain.RaceArchive{}, time.Time{}, false
	}
	return archive, cachedAt, true
}

// parseDriverNumberForRace resolves driver number from query string or cached metadata.
func (h *RaceHandler) parseDriverNumberForRace(r *http.Request, archive domain.RaceArchive) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("driver_number"))
	if raw == "" {
		if archive.Metadata.DriverNumber > 0 {
			return archive.Metadata.DriverNumber, nil
		}
		return 0, fmt.Errorf("missing driver_number query parameter")
	}

	driverNumber, err := strconv.Atoi(raw)
	if err != nil || driverNumber <= 0 {
		return 0, fmt.Errorf("invalid driver_number: %q", raw)
	}
	return driverNumber, nil
}

// parseSnapshotTime parses an optional RFC3339 timestamp query parameter named `at`.
func parseSnapshotTime(r *http.Request) (time.Time, bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("at"))
	if raw == "" {
		return time.Time{}, false, nil
	}

	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid at query parameter %q (use RFC3339)", raw)
	}

	return at.UTC(), true, nil
}

// buildRaceStandings computes one latest position row per driver for the requested snapshot.
func buildRaceStandings(rows []map[string]any, at time.Time, hasAt bool) (time.Time, []raceStandingSnapshot) {
	byDriver := make(map[int]raceStandingSnapshot, 24)
	var snapshotAt time.Time

	for _, row := range rows {
		driverNumber, ok := readIntField(row, "driver_number")
		if !ok || driverNumber <= 0 {
			continue
		}

		position, ok := readIntField(row, "position")
		if !ok || position <= 0 {
			continue
		}

		date, ok := readTimeField(row, "date", "date_start")
		if !ok {
			continue
		}

		if hasAt && date.After(at) {
			continue
		}

		current, found := byDriver[driverNumber]
		if !found || date.After(current.Date) {
			byDriver[driverNumber] = raceStandingSnapshot{
				DriverNumber: driverNumber,
				Position:     position,
				Date:         date,
				Row:          row,
			}
		}

		if date.After(snapshotAt) {
			snapshotAt = date
		}
	}

	standings := make([]raceStandingSnapshot, 0, len(byDriver))
	for _, item := range byDriver {
		standings = append(standings, item)
	}

	sort.Slice(standings, func(i, j int) bool {
		if standings[i].Position == standings[j].Position {
			return standings[i].DriverNumber < standings[j].DriverNumber
		}
		return standings[i].Position < standings[j].Position
	})

	return snapshotAt, standings
}

// indexDriversByNumber maps driver_number to driver profile rows.
func indexDriversByNumber(rows []map[string]any) map[int]map[string]any {
	out := make(map[int]map[string]any, len(rows))
	for _, row := range rows {
		num, ok := readIntField(row, "driver_number")
		if !ok || num <= 0 {
			continue
		}
		out[num] = row
	}
	return out
}

// readTimeField extracts RFC3339 timestamps from a row using one of the given keys.
func readTimeField(row map[string]any, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		v, ok := row[key]
		if !ok || v == nil {
			continue
		}

		switch val := v.(type) {
		case time.Time:
			return val.UTC(), true
		case string:
			parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(val))
			if err != nil {
				continue
			}
			return parsed.UTC(), true
		default:
			continue
		}
	}
	return time.Time{}, false
}

// cloneRow creates a shallow copy of a dataset row to safely enrich response payloads.
func cloneRow(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// filterRowsByDriver returns rows where `driver_number` matches the given driver.
func filterRowsByDriver(rows []map[string]any, driverNumber int) []map[string]any {
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		num, ok := readIntField(row, "driver_number")
		if !ok {
			continue
		}
		if num == driverNumber {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

// readIntField extracts a loosely typed integer field from a dataset row.
func readIntField(row map[string]any, key string) (int, bool) {
	v, ok := row[key]
	if !ok || v == nil {
		return 0, false
	}

	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}
