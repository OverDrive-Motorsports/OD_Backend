/**
##
## OverDrive 2026
## All Technical rights reserved
##
## bootstrap.go - Package bootstrap source file for shared/bootstrap.
##
*/

package bootstrap

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ServiceName string
	Description string
	Version     string
	HTTPPort    string
}

// LoadConfig loads configuration and validates the values required by the service.
func LoadConfig(serviceName string, description string) (Config, error) {
	if err := LoadEnvFiles(serviceName); err != nil {
		return Config{}, err
	}

	httpPort, err := getRequiredEnv("HTTP_PORT")
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServiceName: serviceName,
		Description: description,
		Version:     getEnv("APP_VERSION", "dev"),
		HTTPPort:    httpPort,
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

// LoadEnvFiles loads the root and service-specific .env files when they exist. Both are
// optional: in a container the variables come from the environment, and LoadConfig then checks
// the required ones. A file that exists but cannot be read or parsed is still an error.
func LoadEnvFiles(serviceName string) error {
	for _, path := range []string{".env", filepath.Join("services", serviceName, ".env")} {
		if err := applyDotEnvFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// applyDotEnvFile reads key/value pairs from a .env file into the process environment.
func applyDotEnvFile(path string) error {
	file, err := os.Open(filepath.Clean(path))
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
