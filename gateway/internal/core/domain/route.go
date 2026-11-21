/**
##
## OverDrive 2026
## All Technical rights reserved
##
## route.go - Public route prefixes, their upstream path and their access policy.
##
*/

package domain

import "net/url"

// RouteAccess says who may call a proxied route. The zero value is not a policy: a route
// without an explicit access level is refused when the router is built (fail closed).
type RouteAccess int

const (
	// AccessPublic needs no credential (login, registration, token refresh, OAuth).
	AccessPublic RouteAccess = iota + 1
	// AccessUser needs a valid end-user access token, whatever its role.
	AccessUser
	// AccessAdmin needs an access token with the admin role, or the gateway token for
	// internal and scripted calls. A valid user token without the admin role gets 403.
	AccessAdmin
)

// Route maps a public path prefix to an upstream service. The gateway replaces Prefix with
// UpstreamPrefix in the forwarded path: `/v1/presets/abc` with UpstreamPrefix `/presets`
// reaches the service as `/presets/abc`; an empty UpstreamPrefix strips the prefix.
type Route struct {
	Prefix         string
	UpstreamPrefix string
	Targets        []*url.URL
	Access         RouteAccess
}
