/**
##
## OverDrive 2026
## All Technical rights reserved
##
## security_headers_middleware.go - Baseline HTTP hardening headers on every gateway response.
##
*/

package httpinbound

import "net/http"

// securityHeaders are the baseline hardening headers of every response leaving the gateway.
// The gateway serves JSON only, hence the deny-all content security policy.
var securityHeaders = map[string]string{
	"X-Content-Type-Options":  "nosniff",
	"X-Frame-Options":         "DENY",
	"Referrer-Policy":         "no-referrer",
	"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
}

// SecurityHeaders sets securityHeaders before anything else runs, so the gateway's own
// responses (401, 429, /health, proxy errors) carry them too. On proxied responses the
// upstream copies are dropped by the reverse proxy (see stripSecurityHeaders) to avoid
// duplicated values.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, value := range securityHeaders {
			w.Header().Set(name, value)
		}
		next.ServeHTTP(w, r)
	})
}

// stripSecurityHeaders removes the upstream copies of securityHeaders: the gateway is the
// single authority for them, and httputil.ReverseProxy would otherwise append the upstream
// values to the ones already set (e.g. "X-Frame-Options: DENY, DENY", which browsers reject).
func stripSecurityHeaders(resp *http.Response) error {
	for name := range securityHeaders {
		resp.Header.Del(name)
	}
	return nil
}
