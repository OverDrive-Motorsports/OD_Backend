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

import (
	"net/url"

	"overdrive/gateway/internal/core/domain"
)

// userDataServiceRoutes exposes the presets under /v1/presets, and under the unversioned
// /presets kept for the transition. The service registers its routes under /presets itself, so
// the prefix is forwarded instead of stripped and no other upstream route (such as /health)
// can be reached through it.
func userDataServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: []domain.Route{
			{Prefix: "/v1/presets", UpstreamPrefix: "/presets", Targets: targets.userData, Access: domain.AccessUser},
			{Prefix: "/presets", UpstreamPrefix: "/presets", Targets: targets.userData, Access: domain.AccessUser},
		},
		health: map[string][]*url.URL{
			"/health/user-data": targets.userData,
		},
	}
}
