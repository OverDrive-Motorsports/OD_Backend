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
)

// NewRouter wires all API routes and applies the middleware stack.
func NewRouter(handler *RaceHandler, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()

	// Health and info
	mux.HandleFunc("/", handler.HandleRoot)
	mux.HandleFunc("/health", handler.HandleHealth)

	// Legacy routes expected by current clients.
	mux.HandleFunc("/getrace", handler.HandleGetRace)
	mux.HandleFunc("/sendrace", handler.HandleSendRace)

	// Versioned V1 routes.
	mux.HandleFunc("/api/v1/championships", handler.HandleListChampionships)
	mux.HandleFunc("GET /api/v1/championships/{code}/races", handler.HandleListChampionshipRaces)
	mux.HandleFunc("GET /api/v1/races/{raceId}", handler.HandleGetRaceCatalog)
	mux.HandleFunc("GET /api/v1/races/{raceId}/sessions", handler.HandleListRaceSessions)
	mux.HandleFunc("GET /api/v1/sessions/{sessionId}", handler.HandleGetSessionCatalog)

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
	)
}
