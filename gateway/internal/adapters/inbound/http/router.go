// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"net/http"
	"net/url"
	"overdrive/gateway/internal/core/usecases"
	"strings"
)

func NewHandler(
	routes map[string][]*url.URL,
	serviceHealthRoutes map[string][]*url.URL,
	rateLimitRPS float64,
	rateLimitBurst float64,
	healthUseCase usecases.CheckHealthUseCase,
	authorizeUseCase usecases.AuthorizeRequestUseCase,
) (http.Handler, error) {
	mux := http.NewServeMux()

	healthHandler := NewHealthHandler(healthUseCase)
	protectedGatewayHealth := RequireAuthorization(authorizeUseCase.Execute, http.HandlerFunc(healthHandler.HandleHealth))
	mux.Handle("/health", protectedGatewayHealth)

	for routePath, targets := range serviceHealthRoutes {
		serviceName := strings.TrimPrefix(routePath, "/health/")
		serviceHealthHandler := NewServiceHealthHandler(serviceName, targets)
		protectedServiceHealth := RequireAuthorization(authorizeUseCase.Execute, http.HandlerFunc(serviceHealthHandler.HandleHealth))
		mux.Handle(routePath, protectedServiceHealth)
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

		if prefix == "/v1/race" {
			// Explicit edge validation for race dataset and numeric route parameters.
			serviceHandler = ValidateRaceParameters(serviceHandler)
		}

		protected := RequireAuthorization(authorizeUseCase.Execute, serviceHandler)
		mux.Handle(prefix, protected)
		mux.Handle(prefix+"/", protected)
	}

	// Global middleware order for every route:
	// 1) Rate limit at the edge
	// 2) Request logging on final status
	return RequestLogger(RateLimit(rateLimitRPS, rateLimitBurst, mux)), nil
}
