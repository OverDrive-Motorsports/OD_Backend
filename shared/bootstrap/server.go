/**
##
## OverDrive 2026
## All Technical rights reserved
##
## server.go - HTTP server limits, per-route deadlines and graceful shutdown shared by every binary.
##
*/

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ServerLimits bounds how long a client may take to send a request or read a response, and how
// large its headers may be, so a slow or idle client cannot hold a connection indefinitely.
type ServerLimits struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

// DefaultServerLimits returns the limits used by every service and the gateway. WriteTimeout
// exceeds the 30 s race control long poll; routes that legitimately take longer (replay dump,
// ingestion) extend their own deadline with ExtendDeadlines.
func DefaultServerLimits() ServerLimits {
	return ServerLimits{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

// DefaultShutdownTimeout is how long in-flight requests get to finish after SIGTERM. It exceeds
// the 30 s race control long poll, which the gateway proxies and cannot release early.
const DefaultShutdownTimeout = 35 * time.Second

// NewServer builds an http.Server listening on addr with the given limits.
func NewServer(addr string, handler http.Handler, limits ServerLimits) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: limits.ReadHeaderTimeout,
		ReadTimeout:       limits.ReadTimeout,
		WriteTimeout:      limits.WriteTimeout,
		IdleTimeout:       limits.IdleTimeout,
		MaxHeaderBytes:    limits.MaxHeaderBytes,
	}
}

// ShutdownTimeout reads SHUTDOWN_TIMEOUT (a Go duration such as "30s"), falling back to
// DefaultShutdownTimeout when it is unset. An invalid or non-positive value is an error so a
// typo is caught at startup instead of silently cutting requests.
func ShutdownTimeout() (time.Duration, error) {
	raw := os.Getenv("SHUTDOWN_TIMEOUT")
	if raw == "" {
		return DefaultShutdownTimeout, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration, got %q", raw)
	}
	return value, nil
}

// Serve runs server until SIGINT or SIGTERM, then shuts it down gracefully: the listener closes,
// in-flight requests get up to shutdownTimeout to finish, and Serve returns so the caller's
// deferred cleanup (database disconnect) runs. It returns nil after a clean shutdown.
func Serve(server *http.Server, shutdownTimeout time.Duration) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return Run(ctx, server, nil, shutdownTimeout)
}

// Run serves on listener (or on server.Addr when listener is nil) until ctx is done, then shuts
// the server down within shutdownTimeout. Requests still running at the deadline are cut and
// an error is returned.
func Run(ctx context.Context, server *http.Server, listener net.Listener, shutdownTimeout time.Duration) error {
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", server.Addr)
		if err != nil {
			return err
		}
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("graceful shutdown did not complete within %s: %w", shutdownTimeout, err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// ExtendDeadlines lets one route outlive the server-wide timeouts: the request body may take up
// to read and the response up to write, counted from the moment the handler starts. Use it only
// on routes that need it (long poll, replay dump, ingestion), so every other route keeps the
// short defaults. A zero duration leaves that deadline unchanged.
func ExtendDeadlines(next http.Handler, read time.Duration, write time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller := http.NewResponseController(w)
		now := time.Now()
		if read > 0 {
			_ = controller.SetReadDeadline(now.Add(read))
		}
		if write > 0 {
			_ = controller.SetWriteDeadline(now.Add(write))
		}
		next.ServeHTTP(w, r)
	})
}
