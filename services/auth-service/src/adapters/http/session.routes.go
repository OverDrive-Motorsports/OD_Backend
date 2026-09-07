package httpadapter

import (
	"net/http"
	"overdrive/services/auth-service/src/core/ports"
)

// registerSessionRoutes attaches this module's HTTP handlers to the service router.
// Every route is wrapped in requireOwnUser: these are per-user resources, so a valid
// bearer token alone is not enough — it must authenticate as the {userID} being accessed.
func registerSessionRoutes(mux *http.ServeMux, sessionController *SessionController, authUseCase ports.AuthQueryUseCase) {
	protect := requireOwnUser(authUseCase)
	mux.Handle("GET /{userID}/sessions", protect(http.HandlerFunc(sessionController.GetUserSessions)))
	mux.Handle("GET /{userID}/session/{sessionID}", protect(http.HandlerFunc(sessionController.GetUserSession)))
	mux.Handle("POST /{userID}/session", protect(http.HandlerFunc(sessionController.AddUserSession)))
	mux.Handle("DELETE /{userID}/session/{sessionID}", protect(http.HandlerFunc(sessionController.RemoveUserSession)))
}
