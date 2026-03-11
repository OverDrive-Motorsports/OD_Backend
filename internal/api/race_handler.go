/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_handler.go - Race HTTP handlers for fetch, storage status, and payload delivery endpoints.
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

var publicDatasetOrder = []string{
	"drivers",
	"laps",
	"car_data",
	"location",
	"position",
	"intervals",
	"stints",
	"pit",
	"team_radio",
	"race_control",
	"weather",
	"session_result",
	"starting_grid",
	"overtakes",
	"championship_drivers",
	"championship_teams",
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
			"GET /api/v1/championships",
			"GET /api/v1/championships/{code}/races",
			"GET /api/v1/races/{raceId}",
			"GET /api/v1/races/{raceId}/sessions",
			"GET /api/v1/sessions/{sessionId}",
			"GET /api/v1/race/cache",
			"GET /api/v1/race/storage",
			"GET /api/v1/race/metadata",
			"GET /api/v1/race/datasets",
			"GET /api/v1/race/datasets/{dataset}",
			"GET /api/v1/race/drivers",
			"GET /api/v1/race/drivers/{driverNumber}",
			"GET /api/v1/race/drivers/{driverNumber}/profile",
			"GET /api/v1/race/drivers/{driverNumber}/laps",
			"GET /api/v1/race/drivers/{driverNumber}/telemetry",
			"GET /api/v1/race/drivers/{driverNumber}/location",
			"GET /api/v1/race/drivers/{driverNumber}/position",
			"GET /api/v1/race/drivers/{driverNumber}/intervals",
			"GET /api/v1/race/drivers/{driverNumber}/stints",
			"GET /api/v1/race/drivers/{driverNumber}/pit",
			"GET /api/v1/race/drivers/{driverNumber}/radio",
			"GET /api/v1/race/drivers/{driverNumber}/result",
			"GET /api/v1/race/teams",
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

// HandleListChampionships returns the catalog of stored championships.
func (h *RaceHandler) HandleListChampionships(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := h.raceService.ListChampionships(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list championships: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(items),
		"data":  items,
	})
}

// HandleListChampionshipRaces returns stored races for one championship code.
func (h *RaceHandler) HandleListChampionshipRaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	code := strings.TrimSpace(r.PathValue("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing championship code")
		return
	}

	championship, races, found, err := h.raceService.GetChampionshipRaces(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list championship races: %v", err))
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("championship code=%q not found", code))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"championship": championship,
		"count":        len(races),
		"data":         races,
	})
}

// HandleGetRaceCatalog returns one stored race entry by identifier.
func (h *RaceHandler) HandleGetRaceCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	raceID := strings.TrimSpace(r.PathValue("raceId"))
	if raceID == "" {
		writeError(w, http.StatusBadRequest, "missing raceId path parameter")
		return
	}

	race, found, err := h.raceService.GetRace(r.Context(), raceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read race: %v", err))
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("race id=%q not found", raceID))
		return
	}

	writeJSON(w, http.StatusOK, race)
}

// HandleListRaceSessions returns stored sessions for one race.
func (h *RaceHandler) HandleListRaceSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	raceID := strings.TrimSpace(r.PathValue("raceId"))
	if raceID == "" {
		writeError(w, http.StatusBadRequest, "missing raceId path parameter")
		return
	}

	sessions, found, err := h.raceService.ListRaceSessions(r.Context(), raceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list race sessions: %v", err))
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("race id=%q not found", raceID))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"race_id": raceID,
		"count":   len(sessions),
		"data":    sessions,
	})
}

// HandleGetSessionCatalog returns one stored session entry by identifier.
func (h *RaceHandler) HandleGetSessionCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sessionID := strings.TrimSpace(r.PathValue("sessionId"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "missing sessionId path parameter")
		return
	}

	session, found, err := h.raceService.GetSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read session: %v", err))
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session id=%q not found", sessionID))
		return
	}

	writeJSON(w, http.StatusOK, session)
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

// HandleGetRace fetches race data from the provider and persists it in storage.
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

	archive, err := h.raceService.FetchAndStore(ctx, input)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch race data: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "stored",
		"metadata":     archive.Metadata,
		"counts":       archive.Counts,
		"fetch_errors": archive.FetchErrors,
	})
}

// HandleSendRace returns the latest stored race payload to the caller.
func (h *RaceHandler) HandleSendRace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, storedAt, found, err := h.raceService.GetLatestMergedStored(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read storage: %v", err))
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "no race data stored yet, call GET /getrace first")
		return
	}

	w.Header().Set("X-Stored-At", storedAt.Format(time.RFC3339))
	writeJSON(w, http.StatusOK, archive)
}

// HandleStorageStatus reports whether a race payload is currently stored.
func (h *RaceHandler) HandleStorageStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, storedAt, found, err := h.raceService.GetLatestStored(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read storage: %v", err))
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{
			"stored": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"stored":    true,
		"stored_at": storedAt,
		"metadata":  archive.Metadata,
	})
}

// HandleSendDriverChampionship returns the stored drivers championship standings dataset.
func (h *RaceHandler) HandleSendDriverChampionship(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
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

// HandleSendConstructorChampionship returns the stored constructors championship standings dataset.
func (h *RaceHandler) HandleSendConstructorChampionship(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
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

// HandleSendWeather returns the stored race weather dataset.
func (h *RaceHandler) HandleSendWeather(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
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

	archive, _, ok := h.getMergedArchive(w, r)
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

// HandleSendDriverRace returns all stored race datasets filtered for one driver.
func (h *RaceHandler) HandleSendDriverRace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	driverNumber, err := h.parseDriverNumberForRace(r, archive)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeDriverRacePayload(w, archive, driverNumber)
}

// HandleSendRaceStandings returns race standings snapshots based on in-race position updates.
func (h *RaceHandler) HandleSendRaceStandings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	at, hasAt, err := parseSnapshotTime(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows := archive.Datasets["position"]
	if len(rows) == 0 {
		writeError(w, http.StatusNotFound, "position dataset is empty in stored payload")
		return
	}

	snapshotAt, standings := buildRaceStandings(rows, at, hasAt)
	if len(standings) == 0 {
		if hasAt {
			writeError(w, http.StatusNotFound, fmt.Sprintf("no position rows found at or before %s", at.Format(time.RFC3339)))
			return
		}
		writeError(w, http.StatusNotFound, "no valid position rows found in stored payload")
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

	archive, _, ok := h.getMergedArchive(w, r)
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

// HandleSendMetadata returns meeting and session metadata for the merged active race.
func (h *RaceHandler) HandleSendMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, storedAt, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metadata":     archive.Metadata,
		"meeting":      archive.Meeting,
		"race_session": archive.RaceSession,
		"all_sessions": archive.AllSessions,
		"stored_at":    storedAt,
	})
}

// HandleListDatasets returns the dataset catalog available for the merged active race.
func (h *RaceHandler) HandleListDatasets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	items := make([]map[string]any, 0, len(publicDatasetOrder))
	for _, dataset := range publicDatasetOrder {
		rows, found := archive.Datasets[dataset]
		if !found {
			continue
		}
		items = append(items, map[string]any{
			"dataset": dataset,
			"count":   len(rows),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metadata": archive.Metadata,
		"count":    len(items),
		"data":     items,
	})
}

// HandleSendDataset returns one merged dataset by name.
func (h *RaceHandler) HandleSendDataset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	dataset, ok := resolveDatasetName(r.PathValue("dataset"))
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown dataset")
		return
	}

	data := archive.Datasets[dataset]
	writeJSON(w, http.StatusOK, map[string]any{
		"dataset":  dataset,
		"metadata": archive.Metadata,
		"count":    len(data),
		"data":     data,
	})
}

// HandleListDrivers returns all available drivers for the merged active race.
func (h *RaceHandler) HandleListDrivers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	drivers := sortDriverRows(archive.Datasets["drivers"])
	writeJSON(w, http.StatusOK, map[string]any{
		"dataset":  "drivers",
		"metadata": archive.Metadata,
		"count":    len(drivers),
		"data":     drivers,
	})
}

// HandleListTeams returns teams inferred from the merged drivers dataset.
func (h *RaceHandler) HandleListTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	teams := buildTeams(archive.Datasets["drivers"])
	writeJSON(w, http.StatusOK, map[string]any{
		"dataset":  "teams",
		"metadata": archive.Metadata,
		"count":    len(teams),
		"data":     teams,
	})
}

// HandleSendDriverRaceResource returns the merged full race payload for one driver from a path resource.
func (h *RaceHandler) HandleSendDriverRaceResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	driverNumber, err := parsePathDriverNumber(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeDriverRacePayload(w, archive, driverNumber)
}

// HandleSendDriverProfile returns the merged driver profile only.
func (h *RaceHandler) HandleSendDriverProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	driverNumber, err := parsePathDriverNumber(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	driverInfo := filterRowsByDriver(archive.Datasets["drivers"], driverNumber)
	if len(driverInfo) == 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("driver_number=%d not found in stored race", driverNumber))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metadata":      archive.Metadata,
		"driver_number": driverNumber,
		"driver":        driverInfo[0],
	})
}

// HandleSendDriverDataset returns one driver-scoped dataset from a path resource.
func (h *RaceHandler) HandleSendDriverDataset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	archive, _, ok := h.getMergedArchive(w, r)
	if !ok {
		return
	}

	driverNumber, err := parsePathDriverNumber(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dataset, ok := resolveDatasetName(r.PathValue("dataset"))
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown driver dataset")
		return
	}

	rows := filterRowsByDriver(archive.Datasets[dataset], driverNumber)
	writeJSON(w, http.StatusOK, map[string]any{
		"dataset":       dataset,
		"metadata":      archive.Metadata,
		"driver_number": driverNumber,
		"count":         len(rows),
		"data":          rows,
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

// getStoredArchive loads the latest race archive from storage and writes an HTTP error when unavailable.
func (h *RaceHandler) getStoredArchive(w http.ResponseWriter, r *http.Request) (domain.RaceArchive, time.Time, bool) {
	archive, storedAt, found, err := h.raceService.GetLatestStored(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read storage: %v", err))
		return domain.RaceArchive{}, time.Time{}, false
	}
	if !found {
		writeError(w, http.StatusNotFound, "no race data stored yet, call GET /getrace first")
		return domain.RaceArchive{}, time.Time{}, false
	}
	return archive, storedAt, true
}

// getMergedArchive loads the merged latest race archive from storage and writes an HTTP error when unavailable.
func (h *RaceHandler) getMergedArchive(w http.ResponseWriter, r *http.Request) (domain.RaceArchive, time.Time, bool) {
	archive, storedAt, found, err := h.raceService.GetLatestMergedStored(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read merged storage: %v", err))
		return domain.RaceArchive{}, time.Time{}, false
	}
	if !found {
		writeError(w, http.StatusNotFound, "no race data stored yet, call GET /getrace first")
		return domain.RaceArchive{}, time.Time{}, false
	}
	return archive, storedAt, true
}

// parseDriverNumberForRace resolves driver number from query string or stored metadata.
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

// writeDriverRacePayload writes the merged race payload for one driver.
func (h *RaceHandler) writeDriverRacePayload(w http.ResponseWriter, archive domain.RaceArchive, driverNumber int) {
	driverInfo := filterRowsByDriver(archive.Datasets["drivers"], driverNumber)
	if len(driverInfo) == 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("driver_number=%d not found in stored race", driverNumber))
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

// parsePathDriverNumber parses a positive driver number from the current route path.
func parsePathDriverNumber(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.PathValue("driverNumber"))
	if raw == "" {
		return 0, fmt.Errorf("missing driverNumber path parameter")
	}

	driverNumber, err := strconv.Atoi(raw)
	if err != nil || driverNumber <= 0 {
		return 0, fmt.Errorf("invalid driverNumber: %q", raw)
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

// sortDriverRows orders driver rows by driver_number.
func sortDriverRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, cloneRow(row))
	}

	sort.Slice(out, func(i, j int) bool {
		left, _ := readIntField(out[i], "driver_number")
		right, _ := readIntField(out[j], "driver_number")
		return left < right
	})

	return out
}

// buildTeams derives unique teams from the driver roster.
func buildTeams(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0)
	seen := map[string]struct{}{}

	for _, row := range rows {
		name := strings.TrimSpace(readStringValue(row, "team_name"))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		out = append(out, map[string]any{
			"name":   name,
			"code":   readStringValue(row, "team_code"),
			"colour": readStringValue(row, "team_colour"),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["name"]) < fmt.Sprint(out[j]["name"])
	})

	return out
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

// resolveDatasetName maps public aliases to stored dataset names.
func resolveDatasetName(raw string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(raw))
	aliases := map[string]string{
		"drivers":              "drivers",
		"laps":                 "laps",
		"car_data":             "car_data",
		"telemetry":            "car_data",
		"location":             "location",
		"position":             "position",
		"intervals":            "intervals",
		"stints":               "stints",
		"pit":                  "pit",
		"team_radio":           "team_radio",
		"radio":                "team_radio",
		"race_control":         "race_control",
		"facts":                "race_control",
		"weather":              "weather",
		"session_result":       "session_result",
		"result":               "session_result",
		"starting_grid":        "starting_grid",
		"grid":                 "starting_grid",
		"overtakes":            "overtakes",
		"championship_drivers": "championship_drivers",
		"championship_teams":   "championship_teams",
		"constructors":         "championship_teams",
	}

	dataset, ok := aliases[name]
	return dataset, ok
}

// readStringValue extracts a loosely typed string field from a dataset row.
func readStringValue(row map[string]any, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	default:
		return strings.TrimSpace(fmt.Sprint(val))
	}
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
