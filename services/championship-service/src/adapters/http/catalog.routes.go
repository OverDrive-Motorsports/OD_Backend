/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## catalog.routes.go - Package httpadapter source file for services/championship-service/src/adapters/http.
	##
*/

package httpadapter

import "net/http"

// registerCatalogRoutes attaches this module's HTTP handlers to the service router.
func registerCatalogRoutes(mux *http.ServeMux, catalogController *CatalogController) {
	mux.HandleFunc("GET /championships", catalogController.ListChampionships)
	mux.HandleFunc("GET /championships/{code}/events", catalogController.ListChampionshipEvents)
	mux.HandleFunc("GET /events/{eventId}", catalogController.GetEvent)
	mux.HandleFunc("GET /events/{eventId}/sessions", catalogController.ListEventSessions)
	mux.HandleFunc("GET /sessions/{sessionId}", catalogController.GetSession)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers", catalogController.ListSessionDrivers)
	mux.HandleFunc("GET /sessions/{sessionId}/teams", catalogController.ListSessionTeams)
	mux.HandleFunc("GET /sessions/{sessionId}/datasets/{dataset}", catalogController.GetSessionDataset)
	mux.HandleFunc("GET /sessions/{sessionId}/standings", catalogController.GetSessionStandings)
	mux.HandleFunc("GET /sessions/{sessionId}/standings/race", catalogController.GetSessionRaceStandings)
	mux.HandleFunc("GET /sessions/{sessionId}/broadcast", catalogController.GetSessionBroadcast)
	mux.HandleFunc("GET /drivers/{driverNumber}/profile", catalogController.GetDriverProfile)
}
