/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.routes.go - HTTP route registration for the auth-service health endpoint.
	##
*/
package httpadapter

import "net/http"

func NewRouter(hc *HealthController, ac *AuthController) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", hc.GetHealth)
	mux.HandleFunc("/auth/login", ac.Login)
	mux.HandleFunc("/auth/signup", ac.Signup)
	mux.HandleFunc("/auth/refresh", ac.Refresh)

	return mux
}
