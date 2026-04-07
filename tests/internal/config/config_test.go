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
