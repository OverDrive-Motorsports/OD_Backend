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
	mux.HandleFunc("/api/v1/race/getrace", handler.HandleGetRace)
	mux.HandleFunc("/api/v1/race/sendrace", handler.HandleSendRace)
	mux.HandleFunc("/api/v1/race/cache", handler.HandleCacheStatus)
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
