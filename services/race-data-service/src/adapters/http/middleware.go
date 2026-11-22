/**
##
## OverDrive 2026
## All Technical rights reserved
##
## middleware.go - HTTP middleware shared by race-data-service routes.
##
*/

package httpadapter

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"time"

	"overdrive/shared/apierror"
)

const internalIngestionBodyLimitBytes int64 = 256 << 20

// Internal ingestion batches are large and their storage can take minutes: the route gets its
// own deadlines instead of the server defaults (see bootstrap.ExtendDeadlines).
const (
	internalIngestionReadTimeout  = 5 * time.Minute
	internalIngestionWriteTimeout = 15 * time.Minute
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

// limitRequestBody caps request bodies before JSON decoding.
func limitRequestBody(next http.Handler, limitBytes int64) http.Handler {
	return http.MaxBytesHandler(next, limitBytes)
}

// InternalTokenHeader carries the service-to-service secret on the internal ingestion endpoint.
const InternalTokenHeader = "X-Internal-Token"

// MinInternalTokenBytes is the minimum length of INTERNAL_INGESTION_TOKEN (256 bits).
const MinInternalTokenBytes = 32

// ErrWeakInternalToken is returned by ValidateInternalToken for a missing or too short token.
var ErrWeakInternalToken = errors.New("internal ingestion token is missing or too short")

// ValidateInternalToken reports whether token can protect the internal ingestion endpoint; the
// service refuses to start otherwise, so the endpoint is never left open by mistake.
func ValidateInternalToken(token string) error {
	if len(token) < MinInternalTokenBytes {
		return fmt.Errorf("%w: at least %d bytes required", ErrWeakInternalToken, MinInternalTokenBytes)
	}
	return nil
}

// requireInternalToken lets a request through only when it carries the service-to-service
// secret in InternalTokenHeader (compared in constant time). It runs before the body limit, so
// an unauthenticated caller cannot make the service read a large body.
func requireInternalToken(expected string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(InternalTokenHeader)
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			apierror.Write(w, r.URL.Path, apierror.Unauthorized("invalid internal token", nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}
