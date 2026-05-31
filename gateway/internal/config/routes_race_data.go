// Package config loads environment configuration and route registries.

package config

import "net/url"

func raceDataServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: map[string][]*url.URL{
			"/race-data": targets.raceData,
			"/races":     targets.raceData,
			"/v1/race":   targets.raceData,
		},
		health: map[string][]*url.URL{
			"/health/race-data": targets.raceData,
		},
	}
}
