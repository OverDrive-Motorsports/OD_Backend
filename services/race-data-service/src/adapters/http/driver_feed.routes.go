/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed.routes.go - Registers GET/PUT /sessions/{sessionId}/drivers/{driverNumber}/broadcast.
##
*/

package httpadapter

import "net/http"

// registerDriverFeedRoutes attaches the driver onboard feed handlers to the service router.
func registerDriverFeedRoutes(mux *http.ServeMux, controller *DriverFeedController) {
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/broadcast", controller.GetDriverBroadcast)
	mux.Handle("PUT /sessions/{sessionId}/drivers/{driverNumber}/broadcast", limitRequestBody(
		http.HandlerFunc(controller.PutDriverBroadcast),
		feedListBodyLimitBytes,
	))
}
