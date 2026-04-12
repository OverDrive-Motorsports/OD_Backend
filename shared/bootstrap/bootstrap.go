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

func LoadConfig(serviceName string, description string) (Config, error) {
	if err := loadDotEnv(serviceName); err != nil {
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

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is required and must be defined in the environment or .env", key)
	}

	return value, nil
}

func loadDotEnv(serviceName string) error {
	for _, path := range []string{".env", filepath.Join("services", serviceName, ".env")} {
		if err := applyDotEnvFile(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	return fmt.Errorf("unable to find .env for %s", serviceName)
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
