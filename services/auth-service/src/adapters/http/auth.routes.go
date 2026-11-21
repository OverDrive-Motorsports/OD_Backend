/**
##
## OverDrive 2026
## All Technical rights reserved
##
## auth.routes.go - Registers the public login, registration, refresh and OAuth routes.
##
*/

package httpadapter

import "net/http"

// registerAuthRoutes attaches this module's HTTP handlers to the service router.
func registerAuthRoutes(mux *http.ServeMux, authController *AuthController) {
	mux.HandleFunc("POST /login", authController.Login)
	mux.HandleFunc("POST /register", authController.Register)

	mux.HandleFunc("GET /oauth/providers", authController.ListOAuthProviders)
	mux.HandleFunc("GET /oauth/providers/{providerId}/authorize", authController.OAuthAuthorize)
	mux.HandleFunc("POST /oauth/{providerId}/login", authController.OAuthLogin)
	mux.HandleFunc("POST /oauth/{providerId}/register", authController.OAuthRegister)

	mux.HandleFunc("POST /refresh", authController.Refresh)
}
