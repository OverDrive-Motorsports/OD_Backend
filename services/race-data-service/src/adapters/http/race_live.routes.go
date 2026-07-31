/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_live.routes.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import "net/http"

// registerRaceLiveRoutes attaches this module's HTTP handlers to the service router.
func registerRaceLiveRoutes(mux *http.ServeMux, controller *RaceLiveController) {
	mux.HandleFunc("GET /sessions/{sessionId}/race/position", controller.GetPosition)
	mux.HandleFunc("GET /sessions/{sessionId}/race/laps", controller.GetLaps)
	mux.HandleFunc("GET /sessions/{sessionId}/race/stints", controller.GetStints)
	mux.HandleFunc("GET /sessions/{sessionId}/race/pitstops", controller.GetPitStops)
	mux.HandleFunc("POST /sessions/{sessionId}/race/control", controller.PostRaceControl)
	mux.HandleFunc("GET /sessions/{sessionId}/race/weather", controller.GetWeather)
	mux.HandleFunc("GET /sessions/{sessionId}/race/radio", controller.GetRadio)
}
