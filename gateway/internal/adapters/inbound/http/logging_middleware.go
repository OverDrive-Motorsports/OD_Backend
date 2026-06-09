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

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		log.Printf("%s %s status=%d duration=%s remote=%s", r.Method, r.URL.Path, recorder.statusCode, time.Since(start).Round(time.Millisecond), r.RemoteAddr)
	})
}
