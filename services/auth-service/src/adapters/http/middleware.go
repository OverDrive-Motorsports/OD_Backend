/**
##
## OverDrive 2026
## All Technical rights reserved
##
## middleware.go - HTTP middleware shared by auth-service routes.
##
*/

package httpadapter

import (
	"net/http"
	"overdrive/services/auth-service/src/core/ports"
	"overdrive/shared/apierror"
	"strings"
)

// withSecurityHeaders applies baseline HTTP hardening headers to every response.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}

// requireOwnUser rejects any request whose bearer JWT does not authenticate as the
// {userID} path segment being accessed. Without this, any caller holding a valid JWT for
// ANY account could read/add/remove session records for every OTHER user, since the routes
// only ever validated the shape of the path, never who was actually asking.
func requireOwnUser(authUseCase ports.AuthQueryUseCase) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(authHeader, "Bearer ")
			token = strings.TrimSpace(token)
			if !ok || token == "" {
				apierror.Write(w, r.URL.Path, apierror.Unauthorized("missing bearer token", nil))
				return
			}

			authenticatedUserID, err := authUseCase.VerifyToken(token)
			if err != nil {
				apierror.Write(w, r.URL.Path, apierror.InvalidToken("invalid or expired token", nil))
				return
			}

			if authenticatedUserID != r.PathValue("userID") {
				apierror.Write(w, r.URL.Path, apierror.Forbidden("forbidden", nil))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
