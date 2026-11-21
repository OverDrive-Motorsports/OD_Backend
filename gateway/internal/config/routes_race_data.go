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

import (
	"net/url"

	"overdrive/gateway/internal/core/domain"
)

func raceDataServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: []domain.Route{
			{Prefix: "/v1/race-data", Targets: targets.raceData, Access: domain.AccessUser},
		},
		health: map[string][]*url.URL{
			"/health/race-data": targets.raceData,
		},
	}
}
