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

import (
	"net/url"

	"overdrive/gateway/internal/core/domain"
)

// publicAuthRoutes are the only auth-service routes reachable without a token. Everything else
// under the auth prefixes (the session routes, and any route added later) needs a user access
// token: the public routes are an allow-list, so an unexpected path, such as an encoded slash
// that does not match these patterns, falls under the user rule instead of being public.
var publicAuthRoutes = []string{"/login", "/register", "/refresh", "/oauth"}

// authServiceRoutes exposes auth-service under /v1/auth, and under the unversioned /auth kept
// for the transition. Login, registration, refresh and OAuth are public; every other route
// (the caller's own sessions) needs a user access token.
func authServiceRoutes(targets upstreamTargets) serviceRoutes {
	var proxied []domain.Route
	for _, prefix := range []string{"/v1/auth", "/auth"} {
		proxied = append(proxied, domain.Route{Prefix: prefix, Targets: targets.auth, Access: domain.AccessUser})
		for _, route := range publicAuthRoutes {
			proxied = append(proxied, domain.Route{Prefix: prefix + route, UpstreamPrefix: route, Targets: targets.auth, Access: domain.AccessPublic})
		}
	}
	return serviceRoutes{
		proxied: proxied,
		health: map[string][]*url.URL{
			"/health/auth": targets.auth,
		},
	}
}
