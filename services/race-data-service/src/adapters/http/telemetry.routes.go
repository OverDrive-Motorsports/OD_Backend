/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## telemetry.routes.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import "net/http"

// registerTelemetryRoutes attaches this module's HTTP handlers to the service router.
func registerTelemetryRoutes(mux *http.ServeMux, controller *TelemetryController) {
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/speed", controller.GetSpeed)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/engine", controller.GetEngine)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/location", controller.GetLocation)
	mux.HandleFunc("GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/intervals", controller.GetIntervals)
}
