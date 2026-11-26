/**
##
## OverDrive 2026
## All Technical rights reserved
##
## reverse_proxy_test.go - Unit tests for ReverseProxy path rewriting.
##
*/

package httpinbound

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestJoinPaths(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		extra    string
		want     string
	}{
		{"empty base keeps extra as-is", "", "/foo", "/foo"},
		{"empty base prefixes missing slash", "", "foo", "/foo"},
		{"root base keeps extra as-is", "/", "/foo", "/foo"},
		{"joins base and extra", "/api", "/foo", "/api/foo"},
		{"collapses duplicate slashes", "/api/", "/foo", "/api/foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinPaths(tt.basePath, tt.extra); got != tt.want {
				t.Errorf("joinPaths(%q, %q) = %q, want %q", tt.basePath, tt.extra, got, tt.want)
			}
		})
	}
}

func TestNormalizePrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{"empty becomes root", "", "/"},
		{"adds leading slash", "v1/race-data", "/v1/race-data"},
		{"trims trailing slash", "/v1/race-data/", "/v1/race-data"},
		{"leaves well-formed prefix untouched", "/v1/race-data", "/v1/race-data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePrefix(tt.prefix); got != tt.want {
				t.Errorf("normalizePrefix(%q) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}

// TestReverseProxy_RewritesPathAndStripsPrefix proves the proxy strips the
// route prefix before forwarding to the upstream, and that the upstream's
// own base path (if any) is preserved.
func TestReverseProxy_RewritesPathAndStripsPrefix(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	proxy, err := NewReverseProxy("/v1/race-data", "", []*url.URL{upstreamURL})
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	gateway := httptest.NewServer(proxy)
	defer gateway.Close()

	resp, err := http.Get(gateway.URL + "/v1/race-data/sessions/1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if receivedPath != "/sessions/1" {
		t.Errorf("upstream received path %q, want %q (route prefix should be stripped)", receivedPath, "/sessions/1")
	}
}

// TestReverseProxy_ReplacesPrefixWithUpstreamPrefix proves a route with an upstream prefix
// forwards that prefix instead of stripping everything: the versioned and the transition
// public prefixes both reach the service's own path, the bare prefix and its trailing slash are
// kept apart, and a proxy without any target is refused at construction.
func TestReverseProxy_ReplacesPrefixWithUpstreamPrefix(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)

	cases := []struct {
		routePrefix, upstreamPrefix, path, want string
	}{
		{"/v1/presets", "/presets", "/v1/presets/abc", "/presets/abc"},
		{"/presets", "/presets", "/presets/abc", "/presets/abc"},
		{"/presets", "/presets", "/presets", "/presets"},
		{"/presets", "/presets/", "/presets/", "/presets/"},
		{"/v1/auth/sessions", "/sessions", "/v1/auth/sessions/s-1", "/sessions/s-1"},
		{"/v1/auth", "", "/v1/auth/login", "/login"},
		{"/v1/auth", "", "/v1/auth", "/"},
	}
	for _, tc := range cases {
		proxy, err := NewReverseProxy(tc.routePrefix, tc.upstreamPrefix, []*url.URL{upstreamURL})
		if err != nil {
			t.Fatalf("build proxy: %v", err)
		}
		receivedPath = ""
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK || receivedPath != tc.want {
			t.Fatalf("%s via %s->%q: status = %d, upstream path %q, want %q", tc.path, tc.routePrefix, tc.upstreamPrefix, rec.Code, receivedPath, tc.want)
		}
	}

	if _, err := NewReverseProxy("/v1/presets", "/presets", nil); err == nil {
		t.Fatal("expected an error for a route without upstream target")
	}
}

// TestReverseProxy_ReturnsUpstreamUnavailableOnDial failure proves that a
// dead upstream results in a 502 gateway error, not a hang or a bare 500.
func TestReverseProxy_ReturnsUpstreamUnavailableOnDialFailure(t *testing.T) {
	deadURL, err := url.Parse("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	proxy, err := NewReverseProxy("/v1/race-data", "", []*url.URL{deadURL})
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/race-data/sessions/1", nil)
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

// TestReverseProxy_StripsClientUserID proves a client-supplied X-User-Id, in any case or repeated,
// never reaches the upstream, while its Authorization header does.
func TestReverseProxy_StripsClientUserID(t *testing.T) {
	var got http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	proxy, err := NewReverseProxy("/v1/race-data", "", []*url.URL{target})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/race-data/championships", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Add("x-user-id", "victim")
	req.Header.Add("X-User-ID", "another")
	proxy.ServeHTTP(httptest.NewRecorder(), req)
	if values := got.Values("X-User-Id"); len(values) != 0 {
		t.Fatalf("X-User-Id reached the upstream: %v", values)
	}
	if got.Get("Authorization") != "Bearer token" {
		t.Fatalf("Authorization = %q", got.Get("Authorization"))
	}
}
