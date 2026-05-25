/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.routes.go - Package httpadapter source file for services/ingestion-service/src/adapters/http.
	##
*/

package httpadapter

import "net/http"

// NewRouter builds and returns a router with its required dependencies.
func NewRouter(healthController *HealthController, ingestionController *IngestionController) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthController.GetHealth)
	mux.Handle("POST /providers/openf1/ingestions", limitRequestBody(
		http.HandlerFunc(ingestionController.TriggerOpenF1Ingestion),
		manualIngestionBodyLimitBytes,
	))

	return withSecurityHeaders(mux)
}
