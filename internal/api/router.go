/**
##
## OverDrive 2026
## All Technical rights reserved
##
## router.go - Route registration for V1 and legacy API endpoints.
##
*/

package api

import (
	"log/slog"
	"net/http"
	"overdrive/internal/config"
)

// NewRouter wires all API routes and applies the middleware stack.
func NewRouter(handler *RaceHandler, logger *slog.Logger, cfg config.Config) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	cfg = normalizeSecurityConfig(cfg)

	mux := http.NewServeMux()

	// Health and info
	mux.HandleFunc("/", handler.HandleRoot)
	mux.HandleFunc("/health", handler.HandleHealth)

	// Legacy routes expected by current clients.
	mux.HandleFunc("/getrace", handler.HandleGetRace)
	mux.HandleFunc("/sendrace", handler.HandleSendRace)

	// Versioned V1 routes.
	mux.HandleFunc("/api/v1/championships", handler.HandleListChampionships)
	mux.HandleFunc("GET /api/v1/championships/{code}/events", handler.HandleListChampionshipEvents)
	mux.HandleFunc("GET /api/v1/events/{eventId}", handler.HandleGetEventCatalog)
	mux.HandleFunc("GET /api/v1/events/{eventId}/sessions", handler.HandleListEventSessions)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}", handler.HandleGetSessionCatalog)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/archive", handler.HandleSendSessionArchive)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/metadata", handler.HandleSendSessionMetadata)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/datasets", handler.HandleListSessionDatasets)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/datasets/{dataset}", handler.HandleSendSessionDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers", handler.HandleListSessionDrivers)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/teams", handler.HandleListSessionTeams)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}", handler.HandleSendSessionDriverRace)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/profile", handler.HandleSendSessionDriverProfile)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/broadcast", handler.HandleSendSessionDriverBroadcast)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/laps", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/laps/{lapNumber}/location", handler.HandleSendSessionDriverLapLocation)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/telemetry", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/location", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/position", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/intervals", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/stints", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/pit", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/radio", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/result", handler.HandleSendSessionDriverDataset)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/weather", handler.HandleSendSessionWeather)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/facts", handler.HandleSendSessionFacts)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/standings/race", handler.HandleSendSessionRaceStandings)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}/broadcast", handler.HandleSendSessionBroadcast)

	mux.HandleFunc("/api/v1/race/getrace", handler.HandleGetRace)
	mux.HandleFunc("/api/v1/race/sendrace", handler.HandleSendRace)
	mux.HandleFunc("/api/v1/race/cache", handler.HandleStorageStatus)
	mux.HandleFunc("/api/v1/race/storage", handler.HandleStorageStatus)
	mux.HandleFunc("/api/v1/race/metadata", handler.HandleSendMetadata)
	mux.HandleFunc("/api/v1/race/datasets", handler.HandleListDatasets)
	mux.HandleFunc("GET /api/v1/race/datasets/{dataset}", handler.HandleSendDataset)
	mux.HandleFunc("/api/v1/race/drivers", handler.HandleListDrivers)
	mux.HandleFunc("/api/v1/race/teams", handler.HandleListTeams)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}", handler.HandleSendDriverRaceResource)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/profile", handler.HandleSendDriverProfile)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/laps", handler.HandleSendDriverDataset)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/laps/{lapNumber}/location", handler.HandleSendDriverLapLocation)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/telemetry", handler.HandleSendDriverDataset)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/location", handler.HandleSendDriverDataset)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/position", handler.HandleSendDriverDataset)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/intervals", handler.HandleSendDriverDataset)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/stints", handler.HandleSendDriverDataset)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/pit", handler.HandleSendDriverDataset)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/radio", handler.HandleSendDriverDataset)
	mux.HandleFunc("GET /api/v1/race/drivers/{driverNumber}/result", handler.HandleSendDriverDataset)
	mux.HandleFunc("/api/v1/race/championship/drivers", handler.HandleSendDriverChampionship)
	mux.HandleFunc("/api/v1/race/championship/constructors", handler.HandleSendConstructorChampionship)
	mux.HandleFunc("/api/v1/race/weather", handler.HandleSendWeather)
	mux.HandleFunc("/api/v1/race/facts", handler.HandleSendRaceFacts)
	mux.HandleFunc("/api/v1/race/driver", handler.HandleSendDriverRace)
	mux.HandleFunc("/api/v1/race/standings/race", handler.HandleSendRaceStandings)
	mux.HandleFunc("/api/v1/race/video-url", handler.HandleSendVideoURL)

	return chain(
		mux,
		withRequestID,
		withRecovery(logger),
		withAccessLog(logger),
		withSecurityHeaders,
		withRequestValidation(logger, cfg),
		withCORS(logger, cfg),
		withOptionalAuth(logger, cfg),
		withRateLimit(logger, cfg),
	)
}
