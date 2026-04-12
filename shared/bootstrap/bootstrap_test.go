package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromServiceEnv(t *testing.T) {
	unsetEnv(t, "HTTP_PORT")
	unsetEnv(t, "APP_VERSION")

	root := t.TempDir()
	serviceName := "auth-service"
	serviceDir := filepath.Join(root, "services", serviceName)

	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(serviceDir, ".env"), []byte("HTTP_PORT=3001\nAPP_VERSION=test\n"), 0o644); err != nil {
		t.Fatalf("write .env failed: %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	cfg, err := LoadConfig(serviceName, "Authentication service")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.HTTPPort != "3001" {
		t.Fatalf("expected HTTP port 3001, got %s", cfg.HTTPPort)
	}

	if cfg.Version != "test" {
		t.Fatalf("expected version test, got %s", cfg.Version)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	previousValue, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv failed: %v", err)
	}

	t.Cleanup(func() {
		var err error
		if existed {
			err = os.Setenv(key, previousValue)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restore env failed: %v", err)
		}
	})
}

func TestLoadConfigPrefersExistingEnvironment(t *testing.T) {
	t.Setenv("HTTP_PORT", "9999")
	t.Setenv("APP_VERSION", "env")

	root := t.TempDir()
	serviceName := "auth-service"
	serviceDir := filepath.Join(root, "services", serviceName)

	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(serviceDir, ".env"), []byte("HTTP_PORT=3001\nAPP_VERSION=file\n"), 0o644); err != nil {
		t.Fatalf("write .env failed: %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	cfg, err := LoadConfig(serviceName, "Authentication service")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.HTTPPort != "9999" {
		t.Fatalf("expected HTTP port 9999, got %s", cfg.HTTPPort)
	}

	if cfg.Version != "env" {
		t.Fatalf("expected version env, got %s", cfg.Version)
	}
}
