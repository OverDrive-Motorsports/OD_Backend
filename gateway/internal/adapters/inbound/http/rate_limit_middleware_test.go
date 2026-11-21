/**
##
## OverDrive 2026
## All Technical rights reserved
##
## rate_limit_middleware_test.go - Unit tests for the token-bucket rate limiter.
##
*/

package httpinbound

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLimiter_AllowsUpToBurstThenBlocks(t *testing.T) {
	l := newLimiter(1, 3)

	for i := 0; i < 3; i++ {
		if !l.allow("client-a") {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}

	if l.allow("client-a") {
		t.Fatal("request exceeding burst should be blocked")
	}
}

func TestLimiter_TracksClientsIndependently(t *testing.T) {
	l := newLimiter(1, 1)

	if !l.allow("client-a") {
		t.Fatal("first request for client-a should be allowed")
	}
	if l.allow("client-a") {
		t.Fatal("second immediate request for client-a should be blocked")
	}
	if !l.allow("client-b") {
		t.Fatal("client-b should have its own independent bucket")
	}
}

func TestRateLimit_ReturnsTooManyRequestsWhenExceeded(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(1, 1, nil, next)

	req1 := httptest.NewRequest(http.MethodGet, "/v1/some-route", nil)
	req1.RemoteAddr = "192.0.2.1:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/some-route", nil)
	req2.RemoteAddr = "192.0.2.1:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}

// TestRateLimitKey proves the client is the TCP peer unless that peer is a trusted proxy; only
// then is X-Forwarded-For read, from the right, skipping trusted hops, so a client-written
// left-most value never becomes its identity.
func TestRateLimitKey(t *testing.T) {
	_, proxyNet, _ := net.ParseCIDR("10.0.0.0/8")
	trusted := []*net.IPNet{proxyNet}
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		trusted    []*net.IPNet
		want       string
	}{
		{"no proxy: X-Forwarded-For is ignored", "192.0.2.1:1234", "203.0.113.5", nil, "192.0.2.1"},
		{"untrusted peer: X-Forwarded-For is ignored", "192.0.2.1:1234", "203.0.113.5", trusted, "192.0.2.1"},
		{"trusted peer: right-most untrusted hop", "10.0.0.2:1234", "6.6.6.6, 203.0.113.5", trusted, "203.0.113.5"},
		{"trusted chain is skipped", "10.0.0.2:1234", "203.0.113.5, 10.0.0.9", trusted, "203.0.113.5"},
		{"garbage hop stops the walk", "10.0.0.2:1234", "203.0.113.5, not-an-ip", trusted, "10.0.0.2"},
		{"trusted peer without header", "10.0.0.2:1234", "", trusted, "10.0.0.2"},
		{"proxy line appended separately", "10.0.0.2:1234", "6.6.6.6\n203.0.113.5", trusted, "203.0.113.5"},
		{"remote addr without port", "192.0.2.1", "", nil, "192.0.2.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/some-route", nil)
			req.RemoteAddr = tt.remoteAddr
			for _, line := range strings.Split(tt.forwarded, "\n") {
				if line != "" {
					req.Header.Add("X-Forwarded-For", line) // "\n" separates distinct header lines
				}
			}
			if got := rateLimitKey(req, tt.trusted); got != tt.want {
				t.Errorf("rateLimitKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRateLimit_SpoofedForwardedForDoesNotEscape proves a client sending a different
// X-Forwarded-For on every request is still limited as one client.
func TestRateLimit_SpoofedForwardedForDoesNotEscape(t *testing.T) {
	handler := RateLimit(1, 2, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	codes := []int{}
	for index := 0; index < 3; index++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", index))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("expected the third request to be limited despite a new X-Forwarded-For, got %v", codes)
	}
}

// TestLimiter_EvictsIdleBuckets proves the bucket map stays bounded: at a sweep, buckets idle
// long enough to be full again are dropped, while a bucket used just before the sweep survives
// it.
func TestLimiter_EvictsIdleBuckets(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	now := start
	l := newLimiter(10, 20) // refills completely in 2 s
	l.now = func() time.Time { return now }
	l.lastSweep = start
	for index := 0; index < 1000; index++ {
		l.allow(fmt.Sprintf("client-%d", index))
	}

	now = start.Add(sweepInterval - time.Second)
	l.allow("active") // used 1 s before the sweep: not full again yet

	now = start.Add(sweepInterval)
	l.allow("trigger") // first call at the sweep interval: runs the sweep
	if _, kept := l.clients["active"]; !kept {
		t.Fatal("a bucket used just before the sweep must survive it")
	}
	if len(l.clients) != 2 {
		t.Fatalf("expected only the active and trigger buckets after the sweep, got %d", len(l.clients))
	}
}
