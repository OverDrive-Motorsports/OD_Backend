/**
##
## OverDrive 2026
## All Technical rights reserved
##
## router.go - Builds HTTP routes, middleware chain, and service proxy handlers.
##
*/

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"net/http"
	"net/url"
	"overdrive/gateway/internal/core/usecases"
	"strings"
)

// userTokenRoutePrefixes are the per-user routes: the client authenticates with its own
// access token (JWT issued by auth-service) instead of the shared gateway token, because the
// upstream service needs the caller's identity and both cannot travel in one Authorization
// header. Every other proxied prefix keeps the shared gateway token.
var userTokenRoutePrefixes = map[string]struct{}{
	"/presets": {},
}

func NewHandler(
	routes map[string][]*url.URL,
	serviceHealthRoutes map[string][]*url.URL,
	rateLimitRPS float64,
	rateLimitBurst float64,
	healthUseCase usecases.CheckHealthUseCase,
	authorizeUseCase usecases.AuthorizeRequestUseCase,
	userAuthorizeUseCase usecases.AuthorizeRequestUseCase,
) (http.Handler, error) {
	mux := http.NewServeMux()

	// /health and /health/{service} are intentionally left open (no auth
	// required) so infra/orchestrator liveness probes keep working. Every
	// other route below requires a valid Authorization header: the shared
	// gateway token, or a user JWT on userTokenRoutePrefixes.
	healthHandler := NewHealthHandler(healthUseCase)
	mux.Handle("/health", http.HandlerFunc(healthHandler.HandleHealth))

	for routePath, targets := range serviceHealthRoutes {
		serviceName := strings.TrimPrefix(routePath, "/health/")
		serviceHealthHandler := NewServiceHealthHandler(serviceName, targets)
		mux.Handle(routePath, http.HandlerFunc(serviceHealthHandler.HandleHealth))
	}

	for prefix, targets := range routes {
		proxyHandler, err := NewReverseProxy(prefix, targets)
		if err != nil {
			return nil, err
		}

		serviceHandler := http.Handler(proxyHandler)
		if prefix == "/v1/championship" {
			// Explicit edge validation for dataset enum on public championship v1.
			serviceHandler = ValidateChampionshipDataset(serviceHandler)
		}

		if prefix == "/v1/race-data" {
			// Explicit edge validation for race dataset and numeric route parameters.
			serviceHandler = ValidateRaceParameters(serviceHandler)
		}

		authorize := authorizeUseCase.Execute
		if _, perUser := userTokenRoutePrefixes[prefix]; perUser {
			authorize = userAuthorizeUseCase.Execute
		}
		protected := RequireAuthorization(authorize, serviceHandler)
		mux.Handle(prefix, protected)
		mux.Handle(prefix+"/", protected)
	}

	// Global middleware order for every route:
	// 1) Rate limit at the edge
	// 2) Request logging on final status
	return RequestLogger(RateLimit(rateLimitRPS, rateLimitBurst, mux)), nil
}
