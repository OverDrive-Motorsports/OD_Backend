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

		protected := RequireAuthorization(authorizeUseCase.Execute, proxyHandler)
		mux.Handle(prefix, protected)
		mux.Handle(prefix+"/", protected)
	}

	return RequestLogger(RateLimit(rateLimitRPS, rateLimitBurst, mux)), nil
}
