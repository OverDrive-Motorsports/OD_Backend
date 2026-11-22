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

// authBodyLimitBytes caps the JSON bodies of the public auth routes: credentials, OAuth codes
// and refresh tokens are a few hundred bytes, so anything larger is refused with 413.
const authBodyLimitBytes int64 = 16 << 10

// limitBody wraps a body-reading handler with the auth body size limit.
func limitBody(handler http.HandlerFunc) http.Handler {
	return http.MaxBytesHandler(handler, authBodyLimitBytes)
}

// registerAuthRoutes attaches this module's HTTP handlers to the service router.
func registerAuthRoutes(mux *http.ServeMux, authController *AuthController) {
	mux.Handle("POST /login", limitBody(authController.Login))
	mux.Handle("POST /register", limitBody(authController.Register))

	mux.HandleFunc("GET /oauth/providers", authController.ListOAuthProviders)
	mux.HandleFunc("GET /oauth/providers/{providerId}/authorize", authController.OAuthAuthorize)
	mux.Handle("POST /oauth/{providerId}/login", limitBody(authController.OAuthLogin))
	mux.Handle("POST /oauth/{providerId}/register", limitBody(authController.OAuthRegister))

	mux.Handle("POST /refresh", limitBody(authController.Refresh))
}
