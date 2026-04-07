/**
##
## OverDrive 2026
## All Technical rights reserved
##
## config.go - Environment-driven configuration loader with defaults.
##
*/

package config

import (
	"os"
	"strconv"
	"time"
)

// Config centralise la configuration runtime.
type Config struct {
	OpenF1BaseURL   string
	HTTPTimeout     time.Duration
	RequestInterval time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	LogFormat       string
	APIAddr         string
	GetRaceTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// Load reads backend configuration from environment variables with sane defaults.
func Load() Config {
	return Config{
		OpenF1BaseURL:   getEnv("OPENF1_BASE_URL", "https://api.openf1.org/v1"),
		HTTPTimeout:     getEnvDuration("HTTP_TIMEOUT", 30*time.Second),
		RequestInterval: getEnvDuration("OPENF1_REQUEST_INTERVAL", 350*time.Millisecond),
		MaxRetries:      getEnvInt("OPENF1_MAX_RETRIES", 2),
		RetryDelay:      getEnvDuration("OPENF1_RETRY_DELAY", 1200*time.Millisecond),
		LogFormat:       getEnv("LOG_FORMAT", "pretty"),
		APIAddr:         getEnv("API_ADDR", ":8080"),
		GetRaceTimeout:  getEnvDuration("GETRACE_TIMEOUT", 10*time.Minute),
		ShutdownTimeout: getEnvDuration("API_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

// getEnv returns an environment value or a fallback string when unset.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt returns an environment integer or a fallback when parsing fails.
func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

// getEnvDuration returns an environment duration or a fallback when parsing fails.
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return v
}
