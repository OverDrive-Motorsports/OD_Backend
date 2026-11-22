/**
##
## OverDrive 2026
## All Technical rights reserved
##
## healthcheck.go - Container health probe run by the service binary itself.
##
*/

package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// HealthcheckTimeout bounds the probe, below the Compose healthcheck timeout.
const HealthcheckTimeout = 2 * time.Second

// Healthcheck probes the service listening on 127.0.0.1:port: nil when GET /health answers 200.
// The production images have no shell nor curl, so the container health check runs the
// service binary with the `healthcheck` argument, which calls this and exits non-zero on error.
func Healthcheck(ctx context.Context, port string) error {
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("HTTP_PORT must be a port number for the health check, got %q", port)
	}
	// The target is always the loopback: only the port comes from the environment, re-encoded
	// from its parsed value.
	target := url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(number)), Path: "/health"}
	ctx, cancel := context.WithTimeout(ctx, HealthcheckTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", response.StatusCode)
	}
	return nil
}
