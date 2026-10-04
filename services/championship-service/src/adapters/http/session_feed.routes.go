/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_feed.routes.go - Registers GET/PUT /sessions/{sessionId}/broadcast.
##
*/

package httpadapter

import "net/http"

// registerSessionFeedRoutes attaches the session video feed handlers to the service router.
func registerSessionFeedRoutes(mux *http.ServeMux, controller *SessionFeedController) {
	mux.HandleFunc("GET /sessions/{sessionId}/broadcast", controller.GetSessionBroadcast)
	mux.Handle("PUT /sessions/{sessionId}/broadcast", limitRequestBody(
		http.HandlerFunc(controller.PutSessionBroadcast),
		feedListBodyLimitBytes,
	))
}
