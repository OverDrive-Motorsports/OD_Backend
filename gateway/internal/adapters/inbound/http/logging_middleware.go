/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## logging_middleware.go - Logs inbound HTTP requests with status and duration.
 ##
 */

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"log"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Flush forwards to the underlying ResponseWriter's Flush, if it implements
// http.Flusher. This is required for streaming responses (e.g. Server-Sent
// Events): embedding the http.ResponseWriter INTERFACE only promotes the
// methods declared on that interface (Header/Write/WriteHeader) — it does
// NOT promote Flush(), since that belongs to the separate http.Flusher
// interface, even though the concrete *http.response underneath implements
// it. Without this explicit forward, statusRecorder silently breaks
// httputil.ReverseProxy's immediate flush for text/event-stream responses:
// bytes get buffered until the whole response completes instead of arriving
// frame-by-frame. No current route in this codebase streams a response
// (race-data-service's former /race/replay/stream SSE endpoint was replaced
// by the plain-JSON /race/replay bulk dump), so this forwarding is currently
// unexercised in production — kept in place, and covered by
// flush_passthrough_test.go, as a guard for the next streaming endpoint. Any future
// ResponseWriter wrapper added to this package's middleware chain needs the
// same treatment (Flush and, if ever needed, Hijack/Push/etc.).
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		log.Printf("%s %s status=%d duration=%s remote=%s", r.Method, r.URL.Path, recorder.statusCode, time.Since(start).Round(time.Millisecond), r.RemoteAddr)
	})
}
