/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion.routes.go - Package httpadapter source file for services/championship-service/src/adapters/http.
##
*/

package httpadapter

import (
	"net/http"

	"overdrive/shared/bootstrap"
)

// NewRouter builds and returns a router with its required dependencies. internalToken protects
// POST /internal/ingestion/batches (see requireInternalToken).
func NewRouter(
	healthController *HealthController,
	ingestionController *IngestionController,
	catalogController *CatalogController,
	sessionFeedController *SessionFeedController,
	internalToken string,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthController.GetHealth)
	mux.Handle("POST /internal/ingestion/batches", requireInternalToken(internalToken, bootstrap.ExtendDeadlines(
		limitRequestBody(http.HandlerFunc(ingestionController.ReceiveBatch), internalIngestionBodyLimitBytes),
		internalIngestionReadTimeout,
		internalIngestionWriteTimeout,
	)))
	registerCatalogRoutes(mux, catalogController)
	registerSessionFeedRoutes(mux, sessionFeedController)
	return withSecurityHeaders(mux)
}
