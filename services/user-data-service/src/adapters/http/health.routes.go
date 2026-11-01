/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.routes.go - HTTP route registration for the user-data-service health endpoint.
	##
*/
package httpadapter

import "net/http"

// NewRouter builds the service router. /health stays open for probes; every preset route
// requires a valid user access token.
func NewRouter(controller *HealthController, presetController *PresetController, verifier TokenVerifier) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", controller.GetHealth)
	registerPresetRoutes(mux, presetController, requireUser(verifier))

	return withSecurityHeaders(mux)
}
