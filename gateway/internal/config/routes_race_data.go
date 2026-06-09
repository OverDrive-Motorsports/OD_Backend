/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## routes_race_data.go - Registers proxied and health routes for the race-data service.
 ##
 */

// Package config loads environment configuration and route registries.

package config

import "net/url"

func raceDataServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: map[string][]*url.URL{
			"/v1/race-data": targets.raceData,
		},
		health: map[string][]*url.URL{
			"/health/race-data": targets.raceData,
		},
	}
}
