// Package config loads environment configuration and route registries.

package config

import "net/url"

func championshipServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: map[string][]*url.URL{
			"/championships":    targets.championship,
			"/v1/championship": targets.championship,
		},
		health: map[string][]*url.URL{
			"/health/championship": targets.championship,
		},
	}
}
