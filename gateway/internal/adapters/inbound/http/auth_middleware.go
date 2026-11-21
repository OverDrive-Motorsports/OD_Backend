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

	"overdrive/gateway/internal/core/domain"
	"overdrive/shared/apierror"
)

// RequireUserToken rejects any request the authorize function refuses, with the code matching
// the reason so the client knows what to do: UNAUTHORIZED (no bearer token), TOKEN_EXPIRED
// (call /v1/auth/refresh), INVALID_TOKEN (log in again), FORBIDDEN (valid token whose role does
// not open the route). The 401 codes are the ones the upstream services answer, so the contract
// does not depend on which layer rejects.
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
		case errors.Is(err, domain.ErrForbiddenRole):
			apierror.Write(w, r.URL.Path, apierror.Forbidden("forbidden", nil))
		default:
			apierror.Write(w, r.URL.Path, apierror.InvalidToken("invalid token", nil))
		}
	})
}
