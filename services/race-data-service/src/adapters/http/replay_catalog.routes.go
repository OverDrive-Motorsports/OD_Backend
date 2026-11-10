/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog.routes.go - Registers the replay catalog route.
##
*/

package httpadapter

import "net/http"

// registerReplayCatalogRoutes attaches this module's HTTP handlers to the service router.
func registerReplayCatalogRoutes(mux *http.ServeMux, controller *ReplayCatalogController) {
	mux.HandleFunc("GET /championships/{code}/replays", controller.ListReplays)
}
