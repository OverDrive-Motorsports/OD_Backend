/**
##
## OverDrive 2026
## All Technical rights reserved
##
## routes_registry.go - Aggregates all service route groups into gateway route maps.
##
*/

// Package config loads environment configuration and route registries.

package config

import "net/url"

type serviceRoutes struct {
	proxied map[string][]*url.URL
	health  map[string][]*url.URL
}

func buildRoutes(targets upstreamTargets) map[string][]*url.URL {
	return mergeRouteGroups(allServiceRouteGroups(targets), func(group serviceRoutes) map[string][]*url.URL {
		return group.proxied
	})
}

func buildServiceHealthRoutes(targets upstreamTargets) map[string][]*url.URL {
	return mergeRouteGroups(allServiceRouteGroups(targets), func(group serviceRoutes) map[string][]*url.URL {
		return group.health
	})
}

func allServiceRouteGroups(targets upstreamTargets) []serviceRoutes {
	return []serviceRoutes{
		authServiceRoutes(targets),
		userDataServiceRoutes(targets),
		championshipServiceRoutes(targets),
		raceDataServiceRoutes(targets),
		ingestionServiceRoutes(targets),
	}
}

func mergeRouteGroups(groups []serviceRoutes, selector func(serviceRoutes) map[string][]*url.URL) map[string][]*url.URL {
	merged := map[string][]*url.URL{}
	for _, group := range groups {
		for path, destinations := range selector(group) {
			merged[path] = destinations
		}
	}

	return merged
}
