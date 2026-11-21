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
	"strings"
	"testing"

	"overdrive/gateway/internal/core/domain"
)

// TestBuildRoutes_AccessModel pins the public route table: every prefix, the service path it
// reaches and who may call it. Only login, register, refresh and OAuth are public (the rest of
// the auth prefix, sessions included, needs a user token), data and presets need a user token,
// ingestion is admin-only, the unversioned /auth and /presets mirror their /v1 versions during
// the transition, presets keep their prefix upstream, and nothing else is proxied (no /users
// or /providers, which would land on unrelated upstream routes).
func TestBuildRoutes_AccessModel(t *testing.T) {
	target, _ := url.Parse("http://upstream:3000")
	targets := []*url.URL{target}
	routes := buildRoutes(upstreamTargets{auth: targets, userData: targets, championship: targets, raceData: targets, ingestion: targets})

	type expectation struct {
		upstreamPrefix string
		access         domain.RouteAccess
	}
	want := map[string]expectation{
		"/v1/auth":          {"", domain.AccessUser},
		"/auth":             {"", domain.AccessUser},
		"/v1/auth/login":    {"/login", domain.AccessPublic},
		"/v1/auth/register": {"/register", domain.AccessPublic},
		"/v1/auth/refresh":  {"/refresh", domain.AccessPublic},
		"/v1/auth/oauth":    {"/oauth", domain.AccessPublic},
		"/auth/login":       {"/login", domain.AccessPublic},
		"/auth/register":    {"/register", domain.AccessPublic},
		"/auth/refresh":     {"/refresh", domain.AccessPublic},
		"/auth/oauth":       {"/oauth", domain.AccessPublic},
		"/v1/presets":       {"/presets", domain.AccessUser},
		"/presets":          {"/presets", domain.AccessUser},
		"/v1/championship":  {"", domain.AccessUser},
		"/v1/race-data":     {"", domain.AccessUser},
		"/v1/ingestion":     {"", domain.AccessAdmin},
	}
	if len(routes) != len(want) {
		t.Fatalf("expected %d routes, got %d: %+v", len(want), len(routes), routes)
	}
	for _, route := range routes {
		expected, ok := want[route.Prefix]
		if !ok {
			t.Fatalf("unexpected proxied prefix %q", route.Prefix)
		}
		if route.UpstreamPrefix != expected.upstreamPrefix || route.Access != expected.access || len(route.Targets) != 1 {
			t.Fatalf("%s: got upstream %q access %d targets %d, want %q access %d", route.Prefix, route.UpstreamPrefix, route.Access, len(route.Targets), expected.upstreamPrefix, expected.access)
		}
	}
}

// TestValidateGatewayToken proves the gateway token, an admin credential, must be at least 32
// bytes (surrounding spaces not counted) and that a generated 64-hex-character value passes.
// Load trims the configured value before this check, like the presented token is trimmed
// before comparison, so a value with surrounding spaces still matches.
func TestValidateGatewayToken(t *testing.T) {
	for _, weak := range []string{"", "short-token", strings.Repeat("a", 31), "  " + strings.Repeat("a", 30) + "  "} {
		if err := validateGatewayToken(weak); err == nil {
			t.Fatalf("expected %q to be refused", weak)
		}
	}
	for _, strong := range []string{strings.Repeat("a", 32), strings.Repeat("0f", 32)} {
		if err := validateGatewayToken(strong); err != nil {
			t.Fatalf("expected a %d-byte token to pass, got %v", len(strong), err)
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
