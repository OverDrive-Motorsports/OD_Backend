/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_replay_stream.routes.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import "net/http"

// registerRaceReplayStreamRoutes attaches this module's HTTP handlers to the service router.
func registerRaceReplayStreamRoutes(mux *http.ServeMux, controller *RaceReplayStreamController) {
	mux.HandleFunc("GET /sessions/{sessionId}/race/replay", controller.GetReplay)
}
