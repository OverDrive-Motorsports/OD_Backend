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
	"errors"
	"net/http"
	"strings"

	"overdrive/gateway/internal/core/domain"
	"overdrive/shared/apierror"
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
			apierror.Write(w, r.URL.Path, apierror.Unauthorized("unauthorized", nil))
			return
		}

		if !authorize(authHeader) {
			apierror.Write(w, r.URL.Path, apierror.Unauthorized("unauthorized", nil))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireUserToken rejects any request without a valid end-user access token, with the 401
// code matching the reason so the client knows what to do: UNAUTHORIZED (no bearer token),
// TOKEN_EXPIRED (call /auth/refresh), INVALID_TOKEN (log in again). These are the same codes
// the upstream service would answer, so the contract does not depend on which layer rejects.
func RequireUserToken(authorize func(authHeader string) error, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := authorize(r.Header.Get("Authorization"))
		switch {
		case err == nil:
			next.ServeHTTP(w, r)
		case errors.Is(err, domain.ErrMissingUserToken):
			apierror.Write(w, r.URL.Path, apierror.Unauthorized("missing bearer token", nil))
		case errors.Is(err, domain.ErrUserTokenExpired):
			apierror.Write(w, r.URL.Path, apierror.TokenExpired("token expired", nil))
		default:
			apierror.Write(w, r.URL.Path, apierror.InvalidToken("invalid token", nil))
		}
	})
}
