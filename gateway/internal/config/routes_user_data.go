/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## routes_user_data.go - Registers proxied and health routes for the user-data service.
 ##
 */

// Package config loads environment configuration and route registries.

package config

import "net/url"

func userDataServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: map[string][]*url.URL{
			"/presets":   targets.userData,
			"/providers": targets.userData,
			"/users":     targets.userData,
		},
		health: map[string][]*url.URL{
			"/health/user-data": targets.userData,
		},
	}
}
