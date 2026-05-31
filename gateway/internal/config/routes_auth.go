/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## routes_auth.go - Registers proxied and health routes for the auth service.
 ##
 */

// Package config loads environment configuration and route registries.

package config

import "net/url"

func authServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: map[string][]*url.URL{
			"/auth": targets.auth,
		},
		health: map[string][]*url.URL{
			"/health/auth": targets.auth,
		},
	}
}
