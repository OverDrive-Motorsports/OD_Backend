/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live.routes.go - Registers the live ingestion routes.
##
*/

package httpadapter

import "net/http"

// registerLiveRoutes attaches the live ingestion handlers. They read no body; the gateway
// restricts the whole service to administrators.
func registerLiveRoutes(mux *http.ServeMux, controller *LiveController) {
	mux.HandleFunc("POST /providers/openf1/live/{sessionKey}/start", controller.Start)
	mux.HandleFunc("POST /providers/openf1/live/{sessionKey}/stop", controller.Stop)
	mux.HandleFunc("GET /providers/openf1/live/{sessionKey}/status", controller.Status)
}
