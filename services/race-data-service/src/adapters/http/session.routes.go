/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session.routes.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
##
*/

package httpadapter

import "net/http"

// registerSessionRoutes attaches this module's HTTP handlers to the service router.
func registerSessionRoutes(mux *http.ServeMux, controller *SessionController) {
	mux.HandleFunc("GET /championships", controller.ListChampionships)
	mux.HandleFunc("GET /championships/{code}/events", controller.ListChampionshipEvents)
	mux.HandleFunc("GET /events/{eventId}", controller.GetEvent)
	mux.HandleFunc("GET /events/{eventId}/sessions", controller.ListEventSessions)
	mux.HandleFunc("GET /sessions/{sessionId}", controller.GetSession)
	mux.HandleFunc("GET /sessions/{sessionId}/metadata", controller.GetSessionMetadata)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers", controller.ListSessionDrivers)
	mux.HandleFunc("GET /sessions/{sessionId}/teams", controller.ListSessionTeams)
	mux.HandleFunc("GET /sessions/{sessionId}/datasets/{dataset}", controller.GetSessionDataset)
	mux.HandleFunc("GET /sessions/{sessionId}/standings/race", controller.GetSessionRaceStandings)
	mux.HandleFunc("GET /sessions/{sessionId}/broadcast", controller.GetSessionBroadcast)
	mux.HandleFunc("GET /sessions/{sessionId}/weather", controller.GetSessionWeather)
	mux.HandleFunc("GET /sessions/{sessionId}/facts", controller.GetSessionFacts)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/profile", controller.GetDriverProfile)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/broadcast", controller.GetDriverBroadcast)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/{dataset}", controller.GetDriverDataset)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/laps/{lapNumber}/location", controller.GetDriverLapLocation)
}
