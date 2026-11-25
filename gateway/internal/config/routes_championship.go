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

import (
	"net/http"
	"net/url"

	"overdrive/gateway/internal/core/domain"
)

func championshipServiceRoutes(targets upstreamTargets) serviceRoutes {
	return serviceRoutes{
		proxied: []domain.Route{
			{
				Prefix:  "/v1/championship",
				Targets: targets.championship,
				Access:  domain.AccessUser,
				// Video feeds are shared by every user: only an operator writes them.
				AdminMethods: []string{http.MethodPut},
			},
		},
		health: map[string][]*url.URL{
			"/health/championship": targets.championship,
		},
	}
}
