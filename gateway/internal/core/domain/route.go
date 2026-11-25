/**
##
## OverDrive 2026
## All Technical rights reserved
##
## route.go - Public route prefixes, their upstream path and their access policy.
##
*/

package domain

import (
	"net/url"
	"time"
)

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
	// AdminMethods are the HTTP methods of an AccessUser route that need AccessAdmin instead,
	// whatever the path: shared data any user may read but only an operator may write.
	AdminMethods []string
	// SlowWrite lets some responses of this prefix outlive the server WriteTimeout. The zero
	// value keeps the server default.
	SlowWrite SlowWrite
}

// SlowWrite extends the write deadline of the responses that legitimately take long (an
// ingestion run, the replay dump) without loosening it for every other route.
type SlowWrite struct {
	// PathSuffix limits the extension to request paths ending with it; empty means every path
	// under the route prefix.
	PathSuffix string
	// Timeout is the write deadline, counted from the start of the request.
	Timeout time.Duration
}
