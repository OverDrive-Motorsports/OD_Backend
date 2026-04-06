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
	"strings"
	"time"
)

// Config centralise la configuration runtime.
type Config struct {
	OpenF1BaseURL      string
	HTTPTimeout        time.Duration
	RequestInterval    time.Duration
	MaxRetries         int
	RetryDelay         time.Duration
	LogFormat          string
	CORSAllowedOrigins []string
	RateLimitRequests  int
	RateLimitWindow    time.Duration
	MaxPathLength      int
	MaxQueryLength     int
	JWTSecret          string
	JWTIssuer          string
	JWTAudience        string
	APIAddr            string
	GetRaceTimeout     time.Duration
	ShutdownTimeout    time.Duration
}

// Load reads backend configuration from environment variables with sane defaults.
func Load() Config {
	return Config{
		OpenF1BaseURL:      getEnv("OPENF1_BASE_URL", "https://api.openf1.org/v1"),
		HTTPTimeout:        getEnvDuration("HTTP_TIMEOUT", 30*time.Second),
		RequestInterval:    getEnvDuration("OPENF1_REQUEST_INTERVAL", 350*time.Millisecond),
		MaxRetries:         getEnvInt("OPENF1_MAX_RETRIES", 2),
		RetryDelay:         getEnvDuration("OPENF1_RETRY_DELAY", 1200*time.Millisecond),
		LogFormat:          getEnv("LOG_FORMAT", "pretty"),
		CORSAllowedOrigins: getEnvCSV("CORS_ALLOWED_ORIGINS"),
		RateLimitRequests:  getEnvInt("RATE_LIMIT_REQUESTS", 60),
		RateLimitWindow:    getEnvDuration("RATE_LIMIT_WINDOW", time.Minute),
		MaxPathLength:      getEnvInt("MAX_PATH_LENGTH", 512),
		MaxQueryLength:     getEnvInt("MAX_QUERY_LENGTH", 2048),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTIssuer:          getEnv("JWT_ISSUER", ""),
		JWTAudience:        getEnv("JWT_AUDIENCE", ""),
		APIAddr:            getEnv("API_ADDR", ":8080"),
		GetRaceTimeout:     getEnvDuration("GETRACE_TIMEOUT", 10*time.Minute),
		ShutdownTimeout:    getEnvDuration("API_SHUTDOWN_TIMEOUT", 10*time.Second),
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

func getEnvCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
