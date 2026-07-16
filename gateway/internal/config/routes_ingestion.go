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

import "net/url"

func ingestionServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: map[string][]*url.URL{
			"/v1/ingestion": targets.ingestion,
		},
		health: map[string][]*url.URL{
			"/health/ingestion": targets.ingestion,
		},
	}
}
