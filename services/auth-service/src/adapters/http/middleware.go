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
	"context"
	"net/http"
	"overdrive/services/auth-service/src/core/ports"
	"overdrive/shared/apierror"
	"overdrive/shared/authjwt"
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

type userIDKey struct{}

// requireUser authenticates the caller from `Authorization: Bearer <access token>` and stores
// the user ID (the token's `sub`) in the request context. It is the only source of the user ID
// for the session routes: none reads one from the path, query or body, so a caller can only
// ever act on their own sessions.
func requireUser(authUseCase ports.AuthQueryUseCase) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := authjwt.BearerToken(r.Header.Get("Authorization"))
			if !ok {
				apierror.Write(w, r.URL.Path, apierror.Unauthorized("missing bearer token", nil))
				return
			}

			userID, err := authUseCase.VerifyToken(token)
			if err != nil {
				apierror.Write(w, r.URL.Path, apierror.InvalidToken("invalid or expired token", err))
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, userID)))
		})
	}
}

// userIDFrom returns the authenticated user ID set by requireUser.
func userIDFrom(r *http.Request) string {
	userID, _ := r.Context().Value(userIDKey{}).(string)
	return userID
}
