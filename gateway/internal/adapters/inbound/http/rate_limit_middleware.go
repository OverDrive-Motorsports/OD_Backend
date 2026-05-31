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
)

type clientBucket struct {
	tokens float64
	last   time.Time
}

type limiter struct {
	mu      sync.Mutex
	clients map[string]*clientBucket
	rps     float64
	burst   float64
}

func newLimiter(rps float64, burst float64) *limiter {
	return &limiter{clients: make(map[string]*clientBucket), rps: rps, burst: burst}
}

func (l *limiter) allow(clientID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
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

func RateLimit(rps float64, burst float64, next http.Handler) http.Handler {
	limiter := newLimiter(rps, burst)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(getClientIP(r)) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
