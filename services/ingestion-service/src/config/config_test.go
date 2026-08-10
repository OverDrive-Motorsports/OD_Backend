/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## config_test.go - Package config source file for services/ingestion-service/src/config.
	##
*/

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGetEnv proves getEnv returns the environment value when set, and the fallback otherwise.
func TestGetEnv(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("OD_TEST_GETENV", "value")
		if got := getEnv("OD_TEST_GETENV", "fallback"); got != "value" {
			t.Fatalf("got %q, want value", got)
		}
	})
	t.Run("unset falls back", func(t *testing.T) {
		if got := getEnv("OD_TEST_GETENV_UNSET", "fallback"); got != "fallback" {
			t.Fatalf("got %q, want fallback", got)
		}
	})
}

// TestGetRequiredEnv proves getRequiredEnv returns the value when set, and an explicit error
// (not a silent zero value) when missing.
func TestGetRequiredEnv(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("OD_TEST_REQUIRED", "value")
		got, err := getRequiredEnv("OD_TEST_REQUIRED")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "value" {
			t.Fatalf("got %q, want value", got)
		}
	})
	t.Run("unset returns an error", func(t *testing.T) {
		_, err := getRequiredEnv("OD_TEST_REQUIRED_UNSET")
		if err == nil {
			t.Fatal("expected an error for a missing required var")
		}
	})
}

// TestGetInt proves getInt parses a valid integer, and falls back (rather than erroring) on an
// unset or unparsable value.
func TestGetInt(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("OD_TEST_INT", "7")
		if got := getInt("OD_TEST_INT", 1); got != 7 {
			t.Fatalf("got %d, want 7", got)
		}
	})
	t.Run("unset falls back", func(t *testing.T) {
		if got := getInt("OD_TEST_INT_UNSET", 3); got != 3 {
			t.Fatalf("got %d, want 3", got)
		}
	})
	t.Run("unparsable falls back", func(t *testing.T) {
		t.Setenv("OD_TEST_INT_BAD", "not-a-number")
		if got := getInt("OD_TEST_INT_BAD", 5); got != 5 {
			t.Fatalf("got %d, want 5", got)
		}
	})
}

// TestGetDuration proves getDuration parses a valid duration, and falls back on an unset or
// unparsable value.
func TestGetDuration(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("OD_TEST_DURATION", "2s")
		if got := getDuration("OD_TEST_DURATION", time.Second); got != 2*time.Second {
			t.Fatalf("got %v, want 2s", got)
		}
	})
	t.Run("unset falls back", func(t *testing.T) {
		if got := getDuration("OD_TEST_DURATION_UNSET", 4*time.Second); got != 4*time.Second {
			t.Fatalf("got %v, want 4s", got)
		}
	})
	t.Run("unparsable falls back", func(t *testing.T) {
		t.Setenv("OD_TEST_DURATION_BAD", "not-a-duration")
		if got := getDuration("OD_TEST_DURATION_BAD", 6*time.Second); got != 6*time.Second {
			t.Fatalf("got %v, want 6s", got)
		}
	})
}

// withTempEnvFile chdirs the test into a fresh temp directory containing an empty ".env" file
// (so shared/bootstrap.LoadConfig's dotenv lookup succeeds without requiring the real repo-root
// .env) and restores the original working directory when the test completes.
func withTempEnvFile(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(""), 0o600); err != nil {
		t.Fatalf("failed to write temp .env: %v", err)
	}

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir into temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})
}

// requiredEnv sets every environment variable Load() requires (bootstrap's HTTP_PORT plus this
// service's own RACE_DATA_SERVICE_URL/CHAMPIONSHIP_SERVICE_URL) so tests can selectively omit
// one to exercise the missing-required-var path.
func requiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HTTP_PORT", "3005")
	t.Setenv("RACE_DATA_SERVICE_URL", "http://race-data-service:3004")
	t.Setenv("CHAMPIONSHIP_SERVICE_URL", "http://championship-service:3003")
}

// TestLoad_AllRequiredPresent proves Load succeeds and populates both the shared bootstrap
// config and this service's own required/optional fields when every required var is set.
func TestLoad_AllRequiredPresent(t *testing.T) {
	withTempEnvFile(t)
	requiredEnv(t)
	t.Setenv("OPENF1_RETRY_COUNT", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != "3005" {
		t.Fatalf("HTTPPort = %q, want 3005", cfg.HTTPPort)
	}
	if cfg.RaceDataServiceURL != "http://race-data-service:3004" {
		t.Fatalf("RaceDataServiceURL = %q", cfg.RaceDataServiceURL)
	}
	if cfg.ChampionshipServiceURL != "http://championship-service:3003" {
		t.Fatalf("ChampionshipServiceURL = %q", cfg.ChampionshipServiceURL)
	}
	if cfg.OpenF1RetryCount != 5 {
		t.Fatalf("OpenF1RetryCount = %d, want 5 (env override)", cfg.OpenF1RetryCount)
	}
	if cfg.OpenF1BaseURL != "https://api.openf1.org/v1" {
		t.Fatalf("OpenF1BaseURL = %q, want the default", cfg.OpenF1BaseURL)
	}
}

// TestLoad_MissingRequiredVar proves Load returns an explicit error - rather than a
// zero-valued/silently broken Config - when a service-specific required var is missing, even
// though the shared bootstrap vars are all present.
func TestLoad_MissingRequiredVar(t *testing.T) {
	withTempEnvFile(t)
	t.Setenv("HTTP_PORT", "3005")
	t.Setenv("CHAMPIONSHIP_SERVICE_URL", "http://championship-service:3003")
	// RACE_DATA_SERVICE_URL intentionally left unset.

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when RACE_DATA_SERVICE_URL is missing")
	}
}
