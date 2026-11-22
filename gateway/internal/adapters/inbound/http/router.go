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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"overdrive/gateway/internal/core/domain"
	"overdrive/gateway/internal/core/usecases"
	"strings"
)

func NewHandler(
	routes []domain.Route,
	serviceHealthRoutes map[string][]*url.URL,
	rateLimitRPS float64,
	rateLimitBurst float64,
	trustedProxies []*net.IPNet,
	healthUseCase usecases.CheckHealthUseCase,
	userAuthorizeUseCase usecases.AuthorizeUserRequestUseCase,
	adminAuthorizeUseCase usecases.AuthorizeAdminRequestUseCase,
) (http.Handler, error) {
	mux := http.NewServeMux()

	// /health and /health/{service} are intentionally left open (no auth
	// required) so infra/orchestrator liveness probes keep working. Every
	// proxied route below carries its own access policy (domain.RouteAccess).
	healthHandler := NewHealthHandler(healthUseCase)
	mux.Handle("/health", http.HandlerFunc(healthHandler.HandleHealth))

	for routePath, targets := range serviceHealthRoutes {
		serviceName := strings.TrimPrefix(routePath, "/health/")
		serviceHealthHandler := NewServiceHealthHandler(serviceName, targets)
		mux.Handle(routePath, http.HandlerFunc(serviceHealthHandler.HandleHealth))
	}

	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		prefix := normalizePrefix(route.Prefix)
		if _, duplicate := seen[prefix]; duplicate {
			return nil, fmt.Errorf("route %q is declared twice", prefix)
		}
		seen[prefix] = struct{}{}

		proxyHandler, err := NewReverseProxy(prefix, route.UpstreamPrefix, route.Targets)
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

		var protected http.Handler
		switch route.Access {
		case domain.AccessPublic:
			protected = serviceHandler
		case domain.AccessUser:
			protected = RequireUserToken(userAuthorizeUseCase.Execute, serviceHandler)
		case domain.AccessAdmin:
			protected = RequireUserToken(adminAuthorizeUseCase.Execute, serviceHandler)
		default:
			return nil, fmt.Errorf("route %q has no access policy", prefix)
		}
		if route.SlowWrite.Timeout > 0 {
			protected = ExtendWriteDeadline(route.SlowWrite, protected)
		}
		mux.Handle(prefix, protected)
		mux.Handle(prefix+"/", protected)
	}

	// Global middleware order for every route:
	// 1) Security headers, so every response carries them (including 429 and 401)
	// 2) Request logging on final status
	// 3) Rate limit at the edge
	return SecurityHeaders(RequestLogger(RateLimit(rateLimitRPS, rateLimitBurst, trustedProxies, mux))), nil
}
