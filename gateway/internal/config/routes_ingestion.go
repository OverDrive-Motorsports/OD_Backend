// Package config loads environment configuration and route registries.

package config

import "net/url"

func ingestionServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: map[string][]*url.URL{
			"/ingestion": targets.ingestion,
		},
		health: map[string][]*url.URL{
			"/health/ingestion": targets.ingestion,
		},
	}
}
