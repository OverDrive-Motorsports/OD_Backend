/**
##
## OverDrive 2026
## All Technical rights reserved
##
## security_headers_middleware_test.go - Tests the hardening headers on gateway responses.
##
*/

package httpinbound

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"overdrive/gateway/internal/core/domain"
)

// TestSecurityHeaders_OnGatewayAndProxiedResponses proves the hardening headers are on the
// gateway's own responses (a 401 written before any proxying, /health) and on proxied
// responses - exactly once, even when the upstream already sets them.
func TestSecurityHeaders_OnGatewayAndProxiedResponses(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)

	handler, err := newTestHandler(t, []domain.Route{{Prefix: "/v1/race-data", Targets: []*url.URL{upstreamURL}, Access: domain.AccessUser}})
	if err != nil {
		t.Fatal(err)
	}
	serve := func(path, authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"gateway 401":     serve("/v1/race-data/championships", ""),
		"gateway /health": serve("/health", ""),
		"proxied 200":     serve("/v1/race-data/championships", "Bearer "+signTestJWT(t, "user", time.Minute)),
	} {
		for header, want := range securityHeaders {
			if got := rec.Header().Values(header); len(got) != 1 || got[0] != want {
				t.Fatalf("%s: %s = %v, want exactly [%q]", name, header, got, want)
			}
		}
	}
}
