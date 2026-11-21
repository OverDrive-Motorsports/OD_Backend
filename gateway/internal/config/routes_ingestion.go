/**
##
## OverDrive 2026
## All Technical rights reserved
##
## routes_ingestion.go - Registers proxied and health routes for the ingestion service.
##
*/

// Package config loads environment configuration and route registries.

package config

import (
	"net/url"

	"overdrive/gateway/internal/core/domain"
)

// ingestionServiceRoutes exposes the ingestion trigger to administrators only (admin access
// token, or the gateway token for scripted calls).
func ingestionServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: []domain.Route{
			{Prefix: "/v1/ingestion", Targets: targets.ingestion, Access: domain.AccessAdmin},
		},
		health: map[string][]*url.URL{
			"/health/ingestion": targets.ingestion,
		},
	}
}
