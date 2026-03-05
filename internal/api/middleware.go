/**
##
## OverDrive 2026
## All Technical rights reserved
##
## middleware.go - HTTP middleware chain for request ID, panic recovery, and access logging.
##
*/

package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

var reqCounter uint64

// chain wraps a handler with middlewares in reverse registration order.
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// withRequestID injects or propagates a request ID and exposes it in response headers.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&reqCounter, 1))
		}

		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withRecovery converts panics into HTTP 500 responses and logs the panic context.
func withRecovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered",
						"request_id", requestIDFromContext(r.Context()),
						"method", r.Method,
						"path", r.URL.Path,
						"panic", recovered,
					)
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// withAccessLog writes one structured log entry per HTTP request.
func withAccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			logger.Info("http_request",
				"request_id", requestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// requestIDFromContext returns the request ID attached by the request ID middleware.
func requestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code before forwarding it to the wrapped writer.
func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
