/**
##
## OverDrive 2026
## All Technical rights reserved
##
## auth.routes.go - HTTP route registration for the auth-service endpoints.
##
*/

package httpadapter

import (
	"net/http"
	"overdrive/services/auth-service/src/core/ports"
)

func NewRouter(healthController *HealthController, sessionController *SessionController, authController *AuthController, authUseCase ports.AuthQueryUseCase) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthController.GetHealth)

	registerAuthRoutes(mux, authController)
	registerSessionRoutes(mux, sessionController, authUseCase)
	return withSecurityHeaders(mux)
}
