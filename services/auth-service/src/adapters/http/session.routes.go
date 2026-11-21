/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session.routes.go - Registers the caller's session routes behind requireUser.
##
*/

package httpadapter

import (
	"net/http"
	"overdrive/services/auth-service/src/core/ports"
)

// registerSessionRoutes attaches the caller's own session routes. Every route is wrapped in
// requireUser: the user ID comes from the verified access token only, never from the path.
func registerSessionRoutes(mux *http.ServeMux, sessionController *SessionController, authUseCase ports.AuthQueryUseCase) {
	protect := requireUser(authUseCase)
	mux.Handle("GET /sessions", protect(http.HandlerFunc(sessionController.ListSessions)))
	mux.Handle("GET /sessions/{sessionId}", protect(http.HandlerFunc(sessionController.GetSession)))
	mux.Handle("DELETE /sessions/{sessionId}", protect(http.HandlerFunc(sessionController.DeleteSession)))
}
