/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.routes.go - HTTP route registration for the race-data-service health endpoint.
	##
*/
package httpadapter

import "net/http"

func NewRouter(controller *HealthController) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", controller.GetHealth)

	return mux
}
