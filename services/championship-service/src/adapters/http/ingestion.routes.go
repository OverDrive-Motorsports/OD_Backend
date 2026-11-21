/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion.routes.go - Package httpadapter source file for services/championship-service/src/adapters/http.
##
*/

package httpadapter

import "net/http"

// NewRouter builds and returns a router with its required dependencies. internalToken protects
// POST /internal/ingestion/batches (see requireInternalToken).
func NewRouter(healthController *HealthController, ingestionController *IngestionController, catalogController *CatalogController, internalToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthController.GetHealth)
	mux.Handle("POST /internal/ingestion/batches", requireInternalToken(internalToken, limitRequestBody(
		http.HandlerFunc(ingestionController.ReceiveBatch),
		internalIngestionBodyLimitBytes,
	)))
	registerCatalogRoutes(mux, catalogController)
	return withSecurityHeaders(mux)
}
