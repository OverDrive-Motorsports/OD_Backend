/**
##
## OverDrive 2026
## All Technical rights reserved
##
## config_test.go - Unit tests for environment-driven configuration loading.
##
*/

package config_test

import (
	"testing"
	"time"

	"overdrive/internal/config"
)

// TestLoadDefaults verifies fallback configuration values when env vars are unset.
func TestLoadDefaults(t *testing.T) {
	t.Setenv("OPENF1_BASE_URL", "")
	t.Setenv("HTTP_TIMEOUT", "")
	t.Setenv("OPENF1_REQUEST_INTERVAL", "")
	t.Setenv("OPENF1_MAX_RETRIES", "")
	t.Setenv("OPENF1_RETRY_DELAY", "")
	t.Setenv("LOG_FORMAT", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("RATE_LIMIT_REQUESTS", "")
	t.Setenv("RATE_LIMIT_WINDOW", "")
	t.Setenv("MAX_PATH_LENGTH", "")
	t.Setenv("MAX_QUERY_LENGTH", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_ISSUER", "")
	t.Setenv("JWT_AUDIENCE", "")
	t.Setenv("API_ADDR", "")
	t.Setenv("GETRACE_TIMEOUT", "")
	t.Setenv("API_SHUTDOWN_TIMEOUT", "")

	cfg := config.Load()

	if cfg.OpenF1BaseURL != "https://api.openf1.org/v1" {
		t.Fatalf("unexpected default base url: %s", cfg.OpenF1BaseURL)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Fatalf("unexpected default timeout: %s", cfg.HTTPTimeout)
	}
	if cfg.MaxRetries != 2 {
		t.Fatalf("unexpected default retries: %d", cfg.MaxRetries)
	}
	if cfg.LogFormat != "pretty" {
		t.Fatalf("unexpected default log format: %s", cfg.LogFormat)
	}
	if cfg.RateLimitRequests != 60 {
		t.Fatalf("unexpected default rate limit requests: %d", cfg.RateLimitRequests)
	}
	if cfg.RateLimitWindow != time.Minute {
		t.Fatalf("unexpected default rate limit window: %s", cfg.RateLimitWindow)
	}
	if cfg.MaxPathLength != 512 {
		t.Fatalf("unexpected default max path length: %d", cfg.MaxPathLength)
	}
	if cfg.MaxQueryLength != 2048 {
		t.Fatalf("unexpected default max query length: %d", cfg.MaxQueryLength)
	}
	if len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatalf("unexpected default cors origins: %#v", cfg.CORSAllowedOrigins)
	}
	if cfg.APIAddr != ":8080" {
		t.Fatalf("unexpected default api addr: %s", cfg.APIAddr)
	}
}

// TestLoadOverrides verifies env vars override configuration values when valid.
func TestLoadOverrides(t *testing.T) {
	t.Setenv("OPENF1_BASE_URL", "http://localhost:9000/v1")
	t.Setenv("HTTP_TIMEOUT", "12s")
	t.Setenv("OPENF1_REQUEST_INTERVAL", "150ms")
	t.Setenv("OPENF1_MAX_RETRIES", "5")
	t.Setenv("OPENF1_RETRY_DELAY", "2s")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://beta.overdrive.app")
	t.Setenv("RATE_LIMIT_REQUESTS", "12")
	t.Setenv("RATE_LIMIT_WINDOW", "30s")
	t.Setenv("MAX_PATH_LENGTH", "256")
	t.Setenv("MAX_QUERY_LENGTH", "1024")
	t.Setenv("JWT_SECRET", "top-secret")
	t.Setenv("JWT_ISSUER", "overdrive")
	t.Setenv("JWT_AUDIENCE", "beta-client")
	t.Setenv("API_ADDR", ":9090")
	t.Setenv("GETRACE_TIMEOUT", "15m")
	t.Setenv("API_SHUTDOWN_TIMEOUT", "20s")

	cfg := config.Load()

	if cfg.OpenF1BaseURL != "http://localhost:9000/v1" {
		t.Fatalf("unexpected override base url: %s", cfg.OpenF1BaseURL)
	}
	if cfg.HTTPTimeout != 12*time.Second {
		t.Fatalf("unexpected override timeout: %s", cfg.HTTPTimeout)
	}
	if cfg.RequestInterval != 150*time.Millisecond {
		t.Fatalf("unexpected override request interval: %s", cfg.RequestInterval)
	}
	if cfg.MaxRetries != 5 {
		t.Fatalf("unexpected override retries: %d", cfg.MaxRetries)
	}
	if cfg.RetryDelay != 2*time.Second {
		t.Fatalf("unexpected override retry delay: %s", cfg.RetryDelay)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("unexpected override log format: %s", cfg.LogFormat)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("unexpected cors origins: %#v", cfg.CORSAllowedOrigins)
	}
	if cfg.RateLimitRequests != 12 {
		t.Fatalf("unexpected rate limit requests: %d", cfg.RateLimitRequests)
	}
	if cfg.RateLimitWindow != 30*time.Second {
		t.Fatalf("unexpected rate limit window: %s", cfg.RateLimitWindow)
	}
	if cfg.MaxPathLength != 256 {
		t.Fatalf("unexpected max path length: %d", cfg.MaxPathLength)
	}
	if cfg.MaxQueryLength != 1024 {
		t.Fatalf("unexpected max query length: %d", cfg.MaxQueryLength)
	}
	if cfg.JWTSecret != "top-secret" || cfg.JWTIssuer != "overdrive" || cfg.JWTAudience != "beta-client" {
		t.Fatalf("unexpected jwt config: %#v", cfg)
	}
	if cfg.APIAddr != ":9090" {
		t.Fatalf("unexpected override api addr: %s", cfg.APIAddr)
	}
	if cfg.GetRaceTimeout != 15*time.Minute {
		t.Fatalf("unexpected override getrace timeout: %s", cfg.GetRaceTimeout)
	}
	if cfg.ShutdownTimeout != 20*time.Second {
		t.Fatalf("unexpected override shutdown timeout: %s", cfg.ShutdownTimeout)
	}
}
