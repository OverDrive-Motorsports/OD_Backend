/**
##
## OverDrive 2026
## All Technical rights reserved
##
## middleware.go - Authentication, body-limit and security-header middleware.
##
*/

package httpadapter

import (
	"context"
	"errors"
	"net/http"

	"overdrive/shared/apierror"
	"overdrive/shared/authjwt"
)

// presetBodyLimitBytes caps preset request bodies: the layout document (MaxPresetDataBytes)
// plus room for the other fields and JSON escaping.
const presetBodyLimitBytes int64 = 320 << 10

type userIDKey struct{}

// TokenVerifier is the subset of authjwt.Verifier the middleware needs (faked in tests).
type TokenVerifier interface {
	Verify(token string) (string, error)
}

// requireUser authenticates the caller from `Authorization: Bearer <access token>` and stores
// the user ID (the token's `sub`) in the request context. It is the only source of the user ID:
// no route reads one from the path, query or body. Missing header → 401 UNAUTHORIZED, expired
// token → 401 TOKEN_EXPIRED, any other failure → 401 INVALID_TOKEN.
func requireUser(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := authjwt.BearerToken(r.Header.Get("Authorization"))
			if !ok {
				apierror.Write(w, r.URL.Path, apierror.Unauthorized("missing bearer token", nil))
				return
			}
			userID, err := verifier.Verify(token)
			if err != nil {
				if errors.Is(err, authjwt.ErrTokenExpired) {
					apierror.Write(w, r.URL.Path, apierror.TokenExpired("token expired", err))
					return
				}
				apierror.Write(w, r.URL.Path, apierror.InvalidToken("invalid token", err))
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
