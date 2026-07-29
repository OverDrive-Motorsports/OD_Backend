/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## middleware.go - HTTP middleware shared by championship-service routes.
	##
*/

package httpadapter

import "net/http"

const internalIngestionBodyLimitBytes int64 = 256 << 20

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
