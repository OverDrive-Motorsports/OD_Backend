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
	"strings"
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
	t.Setenv("INTERNAL_INGESTION_TOKEN", "internal-token-long-enough-for-the-check")
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
	if cfg.InternalIngestionToken != "internal-token-long-enough-for-the-check" {
		t.Fatalf("InternalIngestionToken = %q", cfg.InternalIngestionToken)
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

// TestLoad_InternalTokenRequired proves the service refuses to start when
// INTERNAL_INGESTION_TOKEN is missing or shorter than 32 bytes: without it every dispatch
// would be rejected by the receiving services.
func TestLoad_InternalTokenRequired(t *testing.T) {
	for name, value := range map[string]string{"missing": "", "too short": "short-token"} {
		t.Run(name, func(t *testing.T) {
			withTempEnvFile(t)
			requiredEnv(t)
			t.Setenv("INTERNAL_INGESTION_TOKEN", value)
			if _, err := Load(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

// clearLiveEnv unsets the live settings for the test.
func clearLiveEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"OPENF1_USERNAME", "OPENF1_PASSWORD", "OPENF1_TOKEN_URL", "OPENF1_LIVE_POLL_INTERVAL", "OPENF1_LIVE_GRACE", "OPENF1_LIVE_MAX_FAILURES", "OPENF1_LIVE_MAX_JOBS"} {
		t.Setenv(key, "")
	}
}

// TestLoadLiveConfig proves the defaults (no account, documented tuning) and that every setting is
// read; an empty value, as Compose passes for an unset variable, is the default.
func TestLoadLiveConfig(t *testing.T) {
	clearLiveEnv(t)
	cfg, err := loadLiveConfig()
	if err != nil || cfg.OpenF1Username != "" || cfg.OpenF1TokenURL != "https://api.openf1.org/token" ||
		cfg.OpenF1LivePollInterval != 3*time.Second || cfg.OpenF1LiveGrace != 30*time.Minute || cfg.OpenF1LiveMaxFailures != 20 || cfg.OpenF1LiveMaxJobs != 1 {
		t.Fatalf("defaults: %+v err=%v", cfg, err)
	}
	for key, value := range map[string]string{"OPENF1_USERNAME": "u", "OPENF1_PASSWORD": "p", "OPENF1_TOKEN_URL": "https://auth.example.com/token",
		"OPENF1_LIVE_POLL_INTERVAL": "6s", "OPENF1_LIVE_GRACE": "1h", "OPENF1_LIVE_MAX_FAILURES": "5", "OPENF1_LIVE_MAX_JOBS": "2"} {
		t.Setenv(key, value)
	}
	cfg, err = loadLiveConfig()
	if err != nil || cfg.OpenF1Username != "u" || cfg.OpenF1Password != "p" || cfg.OpenF1TokenURL != "https://auth.example.com/token" ||
		cfg.OpenF1LivePollInterval != 6*time.Second || cfg.OpenF1LiveGrace != time.Hour || cfg.OpenF1LiveMaxFailures != 5 || cfg.OpenF1LiveMaxJobs != 2 {
		t.Fatalf("set: %+v err=%v", cfg, err)
	}
}

// TestLoadLiveConfig_Invalid proves half an account, a token URL that would send the password in
// clear and any non-positive or unparsable tuning refuse to start, without echoing a secret.
func TestLoadLiveConfig_Invalid(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"OPENF1_USERNAME", "only"}, {"OPENF1_PASSWORD", "s3cret"},
		{"OPENF1_TOKEN_URL", "http://api.openf1.org/token"}, {"OPENF1_TOKEN_URL", "https://"}, {"OPENF1_TOKEN_URL", "token"},
		{"OPENF1_LIVE_POLL_INTERVAL", "0s"}, {"OPENF1_LIVE_POLL_INTERVAL", "3"}, {"OPENF1_LIVE_GRACE", "-1m"},
		{"OPENF1_LIVE_MAX_FAILURES", "0"}, {"OPENF1_LIVE_MAX_JOBS", "many"},
	} {
		clearLiveEnv(t)
		t.Setenv(tc.key, tc.value)
		if _, err := loadLiveConfig(); err == nil || strings.Contains(err.Error(), "s3cret") {
			t.Fatalf("%s=%s: err=%v", tc.key, tc.value, err)
		}
	}
}

// TestLoad_AccountNeedsHTTPS proves an OpenF1 account refuses a base URL in clear, since its token
// is sent there, while anonymous access does not.
func TestLoad_AccountNeedsHTTPS(t *testing.T) {
	clearLiveEnv(t)
	for key, value := range map[string]string{"HTTP_PORT": "3005", "APP_VERSION": "test", "RACE_DATA_SERVICE_URL": "http://r", "CHAMPIONSHIP_SERVICE_URL": "http://c",
		"INTERNAL_INGESTION_TOKEN": strings.Repeat("a", 32), "OPENF1_BASE_URL": "http://api.openf1.org/v1"} {
		t.Setenv(key, value)
	}
	t.Chdir(t.TempDir())
	if _, err := Load(); err != nil {
		t.Fatalf("anonymous: %v", err)
	}
	t.Setenv("OPENF1_USERNAME", "u")
	t.Setenv("OPENF1_PASSWORD", "p")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error")
	}
	t.Setenv("OPENF1_BASE_URL", "https://api.openf1.org/v1")
	if cfg, err := Load(); err != nil || cfg.OpenF1BaseURL != "https://api.openf1.org/v1" || cfg.OpenF1Username != "u" {
		t.Fatalf("https: %+v %v", cfg, err)
	}
}
