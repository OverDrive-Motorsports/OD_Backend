/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## auth_middleware.go - HTTP middleware enforcing mandatory bearer authorization.
 ##
 */

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"net/http"
	"strings"
)

// RequireAuthorization rejects any request that does not carry a valid
// `Authorization: Bearer <token>` header. Previously requests with NO header
// at all were allowed through (auth was effectively optional), which was a
// real bypass for a public-facing API consumed by AR/mobile clients. Auth is
// now mandatory on every route this middleware wraps; only `/health` is left
// unwrapped by callers so infra probes keep working without a token.
func RequireAuthorization(authorize func(authHeader string) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		if !authorize(authHeader) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
