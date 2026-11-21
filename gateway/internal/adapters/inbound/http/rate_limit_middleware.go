/**
##
## OverDrive 2026
## All Technical rights reserved
##
## rate_limit_middleware.go - Applies token-bucket rate limiting to HTTP requests.
##
*/

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"overdrive/shared/apierror"
)

type clientBucket struct {
	tokens float64
	last   time.Time
}

// sweepInterval bounds how often idle buckets are purged; purging runs inside allow(), so no
// background goroutine is needed.
const sweepInterval = time.Minute

type limiter struct {
	mu        sync.Mutex
	clients   map[string]*clientBucket
	rps       float64
	burst     float64
	lastSweep time.Time
	now       func() time.Time
}

func newLimiter(rps float64, burst float64) *limiter {
	return &limiter{clients: make(map[string]*clientBucket), rps: rps, burst: burst, now: time.Now}
}

func (l *limiter) allow(clientID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)
	bucket, exists := l.clients[clientID]
	if !exists {
		l.clients[clientID] = &clientBucket{tokens: l.burst - 1, last: now}
		return true
	}

	elapsedSeconds := now.Sub(bucket.last).Seconds()
	bucket.last = now
	bucket.tokens = math.Min(l.burst, bucket.tokens+elapsedSeconds*l.rps)
	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens -= 1
	return true
}

// sweep deletes the buckets idle long enough to be full again: a full bucket behaves exactly
// like a missing one, so dropping it changes no decision and keeps the map bounded by the
// clients active during the last refill period. Callers hold l.mu.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < sweepInterval {
		return
	}
	l.lastSweep = now
	refill := time.Duration(l.burst / l.rps * float64(time.Second))
	for id, bucket := range l.clients {
		if now.Sub(bucket.last) >= refill {
			delete(l.clients, id)
		}
	}
}

// RateLimit applies a token bucket per client. The client is identified by its TCP peer
// address; X-Forwarded-For is only read when that peer is one of trustedProxies (see
// rateLimitKey), otherwise any client could pick a new identity per request and never be limited.
func RateLimit(rps float64, burst float64, trustedProxies []*net.IPNet, next http.Handler) http.Handler {
	limiter := newLimiter(rps, burst)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(rateLimitKey(r, trustedProxies)) {
			apierror.Write(w, r.URL.Path, apierror.RateLimitExceeded("rate limit exceeded", nil))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimitKey returns the address the rate limit is keyed on. Without a trusted proxy in front it
// is the TCP peer. When the peer is a trusted proxy, X-Forwarded-For is read from right to
// left, skipping trusted proxies, and the first untrusted address is the client: the right
// end is written by our own proxies, the left end is free text the client controls.
func rateLimitKey(r *http.Request, trustedProxies []*net.IPNet) string {
	peer := clientIP(r.RemoteAddr)
	if !isTrusted(peer, trustedProxies) {
		return peer
	}
	// Every header line counts: some proxies append their own X-Forwarded-For line instead of
	// extending the client's one, and Header.Get would only return the client-written first line.
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for index := len(hops) - 1; index >= 0; index-- {
		hop := strings.TrimSpace(hops[index])
		if net.ParseIP(hop) == nil {
			break
		}
		if !isTrusted(hop, trustedProxies) {
			return hop
		}
	}
	return peer
}

func isTrusted(address string, trustedProxies []*net.IPNet) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}
	for _, network := range trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
