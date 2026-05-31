/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## routes_championship.go - Registers proxied and health routes for the championship service.
 ##
 */

// Package config loads environment configuration and route registries.

package config

import "net/url"

func championshipServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: map[string][]*url.URL{
			"/v1/championship": targets.championship,
		},
		health: map[string][]*url.URL{
			"/health/championship": targets.championship,
		},
	}
}
