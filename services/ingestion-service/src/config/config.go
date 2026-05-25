/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## config.go - Package config source file for services/ingestion-service/src/config.
	##
*/

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"overdrive/shared/bootstrap"
)

type Config struct {
	bootstrap.Config
	OpenF1BaseURL          string
	OpenF1Timeout          time.Duration
	OpenF1RetryCount       int
	OpenF1RetryDelay       time.Duration
	DispatchTimeout        time.Duration
	RaceDataServiceURL     string
	ChampionshipServiceURL string
}

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

	return Config{
		Config:                 base,
		OpenF1BaseURL:          getEnv("OPENF1_BASE_URL", "https://api.openf1.org/v1"),
		OpenF1Timeout:          getDuration("OPENF1_TIMEOUT", 10*time.Second),
		OpenF1RetryCount:       getInt("OPENF1_RETRY_COUNT", 2),
		OpenF1RetryDelay:       getDuration("OPENF1_RETRY_DELAY", 800*time.Millisecond),
		DispatchTimeout:        getDuration("DISPATCH_TIMEOUT", 10*time.Minute),
		RaceDataServiceURL:     raceDataServiceURL,
		ChampionshipServiceURL: championshipServiceURL,
	}, nil
}

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
