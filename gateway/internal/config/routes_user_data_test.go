/**
##
## OverDrive 2026
## All Technical rights reserved
##
## routes_user_data_test.go - Pins the gateway prefixes routed to user-data-service.
##
*/

package config

import (
	"net/url"
	"testing"
)

// TestUserDataServiceRoutes_OnlyPresets proves /presets is the only prefix proxied to
// user-data-service.
func TestUserDataServiceRoutes_OnlyPresets(t *testing.T) {
	target, _ := url.Parse("http://user-data-service:3002")
	routes := userDataServiceRoutes(upstreamTargets{userData: []*url.URL{target}})

	if len(routes.proxied) != 1 || len(routes.proxied["/presets"]) != 1 {
		t.Fatalf("expected /presets as the only user-data prefix, got %v", routes.proxied)
	}
	for _, removed := range []string{"/users", "/providers"} {
		if _, ok := buildRoutes(upstreamTargets{userData: []*url.URL{target}})[removed]; ok {
			t.Fatalf("%s must not be proxied: it would collide with /presets once stripped", removed)
		}
	}
}
