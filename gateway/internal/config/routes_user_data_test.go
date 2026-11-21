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

// TestParseTrustedProxies proves GATEWAY_TRUSTED_PROXIES accepts bare IPv4/IPv6 addresses and
// CIDRs, ignores blanks, treats an empty value as "no proxy", and rejects invalid entries.
func TestParseTrustedProxies(t *testing.T) {
	networks, err := parseTrustedProxies(" 10.0.0.0/8, 192.0.2.7 ,, ::1 ")
	if err != nil || len(networks) != 3 {
		t.Fatalf("expected 3 networks, got %v err=%v", networks, err)
	}
	if networks[1].String() != "192.0.2.7/32" || networks[2].String() != "::1/128" {
		t.Fatalf("bare addresses must become single-host networks, got %v", networks)
	}
	if networks, err := parseTrustedProxies(""); err != nil || len(networks) != 0 {
		t.Fatalf("expected no proxy for an empty value, got %v err=%v", networks, err)
	}
	if _, err := parseTrustedProxies("10.0.0.0/8, not-an-ip"); err == nil {
		t.Fatal("expected an error for an invalid entry")
	}
}
