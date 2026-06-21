/**
##
## OverDrive 2026
## All Technical rights reserved
##
## auth.routes.go - HTTP route registration for the auth-service endpoints.
##
*/

package httpadapter

import "net/http"

func NewRouter(healthController *HealthController, sessionController *SessionController) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthController.GetHealth)

	registerSessionRoutes(mux, sessionController)
	return withSecurityHeaders(mux)
}
