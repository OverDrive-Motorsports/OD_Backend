/**
##
## OverDrive 2026
## All Technical rights reserved
##
## config.go - Loads environment configuration and upstream service targets.
##
*/

// Package config loads environment configuration and route registries.

package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	HTTPPort            string
	AuthToken           string
	RateLimitRPS        float64
	RateLimitBurst      float64
	Routes              map[string][]*url.URL
	ServiceHealthRoutes map[string][]*url.URL
}

type upstreamTargets struct {
	auth         []*url.URL
	userData     []*url.URL
	championship []*url.URL
	raceData     []*url.URL
	ingestion    []*url.URL
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	httpPort, err := getRequiredEnv("HTTP_PORT")
	if err != nil {
		return Config{}, err
	}

	authToken, err := getRequiredEnv("GATEWAY_AUTH_TOKEN")
	if err != nil {
		return Config{}, err
	}

	rps, err := parsePositiveFloat("GATEWAY_RATE_LIMIT_RPS")
	if err != nil {
		return Config{}, err
	}

	burst, err := parsePositiveFloat("GATEWAY_RATE_LIMIT_BURST")
	if err != nil {
		return Config{}, err
	}

	targets, err := buildUpstreamTargets()
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPPort:            httpPort,
		AuthToken:           authToken,
		RateLimitRPS:        rps,
		RateLimitBurst:      burst,
		Routes:              buildRoutes(targets),
		ServiceHealthRoutes: buildServiceHealthRoutes(targets),
	}, nil
}

func buildUpstreamTargets() (upstreamTargets, error) {
	authRaw, err := getRequiredEnv("AUTH_SERVICE_URLS")
	if err != nil {
		return upstreamTargets{}, err
	}
	authTargets, err := parseTargets(authRaw)
	if err != nil {
		return upstreamTargets{}, fmt.Errorf("AUTH_SERVICE_URLS: %w", err)
	}

	userDataRaw, err := getRequiredEnv("USER_DATA_SERVICE_URLS")
	if err != nil {
		return upstreamTargets{}, err
	}
	userDataTargets, err := parseTargets(userDataRaw)
	if err != nil {
		return upstreamTargets{}, fmt.Errorf("USER_DATA_SERVICE_URLS: %w", err)
	}

	championshipRaw, err := getRequiredEnv("CHAMPIONSHIP_SERVICE_URLS")
	if err != nil {
		return upstreamTargets{}, err
	}
	championshipTargets, err := parseTargets(championshipRaw)
	if err != nil {
		return upstreamTargets{}, fmt.Errorf("CHAMPIONSHIP_SERVICE_URLS: %w", err)
	}

	raceDataRaw, err := getRequiredEnv("RACE_DATA_SERVICE_URLS")
	if err != nil {
		return upstreamTargets{}, err
	}
	raceDataTargets, err := parseTargets(raceDataRaw)
	if err != nil {
		return upstreamTargets{}, fmt.Errorf("RACE_DATA_SERVICE_URLS: %w", err)
	}

	ingestionRaw, err := getRequiredEnv("INGESTION_SERVICE_URLS")
	if err != nil {
		return upstreamTargets{}, err
	}
	ingestionTargets, err := parseTargets(ingestionRaw)
	if err != nil {
		return upstreamTargets{}, fmt.Errorf("INGESTION_SERVICE_URLS: %w", err)
	}

	return upstreamTargets{
		auth:         authTargets,
		userData:     userDataTargets,
		championship: championshipTargets,
		raceData:     raceDataTargets,
		ingestion:    ingestionTargets,
	}, nil
}

func parseTargets(raw string) ([]*url.URL, error) {
	parts := strings.Split(raw, ",")
	targets := make([]*url.URL, 0, len(parts))

	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}

		parsed, err := url.Parse(candidate)
		if err != nil {
			return nil, err
		}

		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("invalid target %q", candidate)
		}

		targets = append(targets, parsed)
	}

	if len(targets) == 0 {
		return nil, errors.New("at least one target is required")
	}

	return targets, nil
}

func parsePositiveFloat(key string) (float64, error) {
	raw, err := getRequiredEnv(key)
	if err != nil {
		return 0, err
	}

	value, parseErr := strconv.ParseFloat(raw, 64)
	if parseErr != nil {
		return 0, fmt.Errorf("%s must be a number", key)
	}

	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}

	return value, nil
}

func getRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is required and must be defined in gateway/.env or the environment", key)
	}

	return value, nil
}

func loadDotEnv() error {
	paths := []string{filepath.Join("gateway", ".env")}

	if cwd, err := os.Getwd(); err == nil && filepath.Base(cwd) == "gateway" {
		paths = append(paths, ".env")
	}

	loadedAtLeastOne := false
	for _, path := range paths {
		if err := applyDotEnvFile(path); err == nil {
			loadedAtLeastOne = true
			break
		} else if errors.Is(err, os.ErrNotExist) {
			continue
		} else {
			return err
		}
	}

	if !loadedAtLeastOne {
		return fmt.Errorf("unable to find gateway .env (expected %s)", strings.Join(paths, " or "))
	}

	return nil
}

func applyDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("invalid .env line in %s: %q", path, line)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("invalid empty key in %s", path)
		}

		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}
