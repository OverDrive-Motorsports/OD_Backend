package httpadapter

import "net/http"

// registerSessionRoutes attaches this module's HTTP handlers to the service router.
func registerSessionRoutes(mux *http.ServeMux, sessionController *SessionController) {
	mux.HandleFunc("GET /{userID}/sessions", sessionController.GetUserSessions)
	mux.HandleFunc("GET /{userID}/session/{sessionID}", sessionController.GetUserSession)
	mux.HandleFunc("POST /{userID}/session", sessionController.AddUserSession)
	mux.HandleFunc("DELETE /{userID}/session", sessionController.RemoveUserSession)
}
