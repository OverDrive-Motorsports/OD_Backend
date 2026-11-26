/**
##
## OverDrive 2026
## All Technical rights reserved
##
## config.go - Package config source file for services/ingestion-service/src/config.
##
*/

package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"overdrive/shared/bootstrap"
)

type Config struct {
	bootstrap.Config
	OpenF1BaseURL    string
	OpenF1Timeout    time.Duration
	OpenF1RetryCount int
	OpenF1RetryDelay time.Duration
	// OpenF1MinInterval is the minimum time between two OpenF1 requests.
	OpenF1MinInterval time.Duration
	// OpenF1MaxResponseBytes bounds one OpenF1 response.
	OpenF1MaxResponseBytes int64
	// OpenF1Username and OpenF1Password are the optional OpenF1 account, required by OpenF1 for
	// live data. Both or neither.
	OpenF1Username string
	OpenF1Password string
	OpenF1TokenURL string
	// Live job tuning (see usecases.LiveOptions).
	OpenF1LivePollInterval time.Duration
	OpenF1LiveGrace        time.Duration
	OpenF1LiveMaxFailures  int
	OpenF1LiveMaxJobs      int
	DispatchTimeout        time.Duration
	RaceDataServiceURL     string
	ChampionshipServiceURL string
	// InternalIngestionToken is sent on every dispatch; championship-service and
	// race-data-service reject batches without it.
	InternalIngestionToken string
}

// minInternalTokenBytes is the minimum length of INTERNAL_INGESTION_TOKEN (256 bits); the
// receiving services enforce the same minimum.
const minInternalTokenBytes = 32

// Load loads configuration and validates the values required by the service.
func Load() (Config, error) {
	base, err := bootstrap.LoadConfig(
		"ingestion-service",
		"External provider ingestion and synchronization service.",
	)
	if err != nil {
		return Config{}, err
	}

	raceDataServiceURL, err := getRequiredEnv("RACE_DATA_SERVICE_URL")
	if err != nil {
		return Config{}, err
	}
	championshipServiceURL, err := getRequiredEnv("CHAMPIONSHIP_SERVICE_URL")
	if err != nil {
		return Config{}, err
	}
	internalToken, err := getRequiredEnv("INTERNAL_INGESTION_TOKEN")
	if err != nil {
		return Config{}, err
	}
	if len(internalToken) < minInternalTokenBytes {
		return Config{}, fmt.Errorf("INTERNAL_INGESTION_TOKEN must be at least %d bytes", minInternalTokenBytes)
	}

	live, err := loadLiveConfig()
	if err != nil {
		return Config{}, err
	}
	openF1BaseURL := getEnv("OPENF1_BASE_URL", "https://api.openf1.org/v1")
	if live.OpenF1Username != "" && !isHTTPS(openF1BaseURL) {
		return Config{}, errors.New("OPENF1_BASE_URL must be an https URL when an OpenF1 account is set: its token is sent to it")
	}

	return Config{
		Config:                 base,
		OpenF1BaseURL:          openF1BaseURL,
		OpenF1Timeout:          getDuration("OPENF1_TIMEOUT", 10*time.Second),
		OpenF1RetryCount:       getInt("OPENF1_RETRY_COUNT", 2),
		OpenF1RetryDelay:       getDuration("OPENF1_RETRY_DELAY", 800*time.Millisecond),
		OpenF1MinInterval:      getDuration("OPENF1_MIN_REQUEST_INTERVAL", 350*time.Millisecond),
		OpenF1MaxResponseBytes: int64(getInt("OPENF1_MAX_RESPONSE_BYTES", int(defaultOpenF1MaxResponseBytes))),
		DispatchTimeout:        getDuration("DISPATCH_TIMEOUT", 10*time.Minute),
		RaceDataServiceURL:     raceDataServiceURL,
		ChampionshipServiceURL: championshipServiceURL,
		InternalIngestionToken: internalToken,
		OpenF1Username:         live.OpenF1Username,
		OpenF1Password:         live.OpenF1Password,
		OpenF1TokenURL:         live.OpenF1TokenURL,
		OpenF1LivePollInterval: live.OpenF1LivePollInterval,
		OpenF1LiveGrace:        live.OpenF1LiveGrace,
		OpenF1LiveMaxFailures:  live.OpenF1LiveMaxFailures,
		OpenF1LiveMaxJobs:      live.OpenF1LiveMaxJobs,
	}, nil
}

// isHTTPS reports whether raw is an absolute https URL.
func isHTTPS(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

// loadLiveConfig reads the OpenF1 account and the live job tuning. An incomplete account or a
// non-positive value is an error: a typo must not surface in the middle of a live session. An
// empty value, as Compose passes for an unset variable, is the default.
func loadLiveConfig() (Config, error) {
	cfg := Config{
		OpenF1Username:         os.Getenv("OPENF1_USERNAME"),
		OpenF1Password:         os.Getenv("OPENF1_PASSWORD"),
		OpenF1TokenURL:         getEnv("OPENF1_TOKEN_URL", "https://api.openf1.org/token"),
		OpenF1LivePollInterval: 3 * time.Second,
		OpenF1LiveGrace:        30 * time.Minute,
		OpenF1LiveMaxFailures:  20,
		OpenF1LiveMaxJobs:      1,
	}
	if (cfg.OpenF1Username == "") != (cfg.OpenF1Password == "") {
		return Config{}, errors.New("OPENF1_USERNAME and OPENF1_PASSWORD must be set together")
	}
	if !isHTTPS(cfg.OpenF1TokenURL) {
		return Config{}, errors.New("OPENF1_TOKEN_URL must be an https URL: the account password is sent to it")
	}
	for key, target := range map[string]*time.Duration{"OPENF1_LIVE_POLL_INTERVAL": &cfg.OpenF1LivePollInterval, "OPENF1_LIVE_GRACE": &cfg.OpenF1LiveGrace} {
		if raw := os.Getenv(key); raw != "" {
			value, err := time.ParseDuration(raw)
			if err != nil || value <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive duration", key)
			}
			*target = value
		}
	}
	for key, target := range map[string]*int{"OPENF1_LIVE_MAX_FAILURES": &cfg.OpenF1LiveMaxFailures, "OPENF1_LIVE_MAX_JOBS": &cfg.OpenF1LiveMaxJobs} {
		if raw := os.Getenv(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive integer", key)
			}
			*target = value
		}
	}
	return cfg, nil
}

// defaultOpenF1MaxResponseBytes bounds one OpenF1 response (one driver's car_data is a few MB).
const defaultOpenF1MaxResponseBytes int64 = 128 << 20

// getEnv returns an environment value or the provided fallback.
func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

// getRequiredEnv returns a required environment value or an explicit error.
func getRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is required and must be defined in the environment or .env", key)
	}
	return value, nil
}

// getInt reads an integer environment value, falling back when parsing fails.
func getInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

// getDuration reads a duration environment value, falling back when parsing fails.
func getDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}
