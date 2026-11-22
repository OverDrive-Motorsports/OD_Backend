/**
##
## OverDrive 2026
## All Technical rights reserved
##
## bootstrap_test.go - Unit tests for the service configuration loader.
##
*/

package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configKeys are the variables the tests read or write. They are cleared for the test and
// restored afterwards, including the ones the loader sets itself with os.Setenv.
var configKeys = []string{"HTTP_PORT", "APP_VERSION", "BOOTSTRAP_TEST_EXTRA", "BOOTSTRAP_TEST_URL"}

// isolateConfig runs the test in an empty directory with the config variables unset.
func isolateConfig(t *testing.T) string {
	t.Helper()
	for _, key := range configKeys {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

// writeFile creates path (and its parents) under dir with content.
func writeFile(t *testing.T, dir string, path string, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestLoadConfig_RootEnvFile proves a root .env alone is enough: its values are loaded and
// APP_VERSION falls back to "dev".
func TestLoadConfig_RootEnvFile(t *testing.T) {
	dir := isolateConfig(t)
	writeFile(t, dir, ".env", "HTTP_PORT=3002\n")

	cfg, err := LoadConfig("user-data-service", "presets")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := Config{ServiceName: "user-data-service", Description: "presets", Version: "dev", HTTPPort: "3002"}
	if cfg != want {
		t.Fatalf("config = %+v, want %+v", cfg, want)
	}
}

// TestLoadConfig_ServiceEnvFile proves services/<name>/.env alone is enough, and that
// APP_VERSION is read when set.
func TestLoadConfig_ServiceEnvFile(t *testing.T) {
	dir := isolateConfig(t)
	writeFile(t, dir, "services/auth-service/.env", "HTTP_PORT=3001\nAPP_VERSION=1.2.3\n")

	cfg, err := LoadConfig("auth-service", "auth")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HTTPPort != "3001" || cfg.Version != "1.2.3" {
		t.Fatalf("config = %+v", cfg)
	}
}

// TestLoadConfig_RootFileWinsOverServiceFile proves the first value wins: the root .env is
// read first, so a key it defines is not replaced by the service file, while keys only the
// service file defines are still loaded.
func TestLoadConfig_RootFileWinsOverServiceFile(t *testing.T) {
	dir := isolateConfig(t)
	writeFile(t, dir, ".env", "HTTP_PORT=3000\n")
	writeFile(t, dir, "services/auth-service/.env", "HTTP_PORT=3999\nBOOTSTRAP_TEST_EXTRA=from-service\n")

	cfg, err := LoadConfig("auth-service", "auth")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HTTPPort != "3000" {
		t.Fatalf("HTTP_PORT = %q, want the root value 3000", cfg.HTTPPort)
	}
	if got := os.Getenv("BOOTSTRAP_TEST_EXTRA"); got != "from-service" {
		t.Fatalf("BOOTSTRAP_TEST_EXTRA = %q, want the service file value", got)
	}
}

// TestLoadConfig_EnvironmentWinsOverFiles proves a variable already set in the process (by
// Compose or the shell) is never replaced by a .env value.
func TestLoadConfig_EnvironmentWinsOverFiles(t *testing.T) {
	dir := isolateConfig(t)
	t.Setenv("HTTP_PORT", "4000")
	writeFile(t, dir, ".env", "HTTP_PORT=3000\n")

	cfg, err := LoadConfig("auth-service", "auth")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HTTPPort != "4000" {
		t.Fatalf("HTTP_PORT = %q, want the environment value 4000", cfg.HTTPPort)
	}
}

// TestLoadConfig_NoEnvFile pins the current behaviour: without any .env file the service
// refuses to start, even when the environment already holds every variable.
func TestLoadConfig_NoEnvFile(t *testing.T) {
	isolateConfig(t)
	t.Setenv("HTTP_PORT", "3001")

	_, err := LoadConfig("auth-service", "auth")
	if err == nil || !strings.Contains(err.Error(), "unable to find .env for auth-service") {
		t.Fatalf("err = %v, want the missing .env error", err)
	}
}

// TestLoadConfig_MissingHTTPPort proves HTTP_PORT is required and the error names it.
func TestLoadConfig_MissingHTTPPort(t *testing.T) {
	dir := isolateConfig(t)
	writeFile(t, dir, ".env", "APP_VERSION=1\n")

	_, err := LoadConfig("auth-service", "auth")
	if err == nil || !strings.Contains(err.Error(), "HTTP_PORT is required") {
		t.Fatalf("err = %v, want the HTTP_PORT requirement", err)
	}
}

// TestLoadConfig_EmptyHTTPPort proves an empty value counts as missing.
func TestLoadConfig_EmptyHTTPPort(t *testing.T) {
	dir := isolateConfig(t)
	writeFile(t, dir, ".env", "HTTP_PORT=\n")

	if _, err := LoadConfig("auth-service", "auth"); err == nil || !strings.Contains(err.Error(), "HTTP_PORT is required") {
		t.Fatalf("err = %v, want the HTTP_PORT requirement", err)
	}
}

// TestApplyDotEnvFile_Syntax proves the parser skips blank lines and comments, trims spaces
// around keys and values, and keeps any "=" after the first one in the value (database URLs
// carry query strings).
func TestApplyDotEnvFile_Syntax(t *testing.T) {
	dir := isolateConfig(t)
	writeFile(t, dir, ".env", "\n# comment\n   \n  HTTP_PORT  =  3005  \nBOOTSTRAP_TEST_URL=postgresql://u:p@h/db?sslmode=disable&x=1\n")

	if err := applyDotEnvFile(".env"); err != nil {
		t.Fatalf("applyDotEnvFile: %v", err)
	}
	if got := os.Getenv("HTTP_PORT"); got != "3005" {
		t.Fatalf("HTTP_PORT = %q", got)
	}
	if got := os.Getenv("BOOTSTRAP_TEST_URL"); got != "postgresql://u:p@h/db?sslmode=disable&x=1" {
		t.Fatalf("BOOTSTRAP_TEST_URL = %q", got)
	}
}

// TestLoadConfig_InvalidLines proves a malformed .env stops the start with an error naming
// the file, instead of silently ignoring the line.
func TestLoadConfig_InvalidLines(t *testing.T) {
	cases := map[string]struct {
		content string
		want    string
	}{
		"line without equals": {"HTTP_PORT=3001\nNOT_A_PAIR\n", `invalid .env line in .env: "NOT_A_PAIR"`},
		"empty key":           {"HTTP_PORT=3001\n = value\n", "invalid empty key in .env"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := isolateConfig(t)
			writeFile(t, dir, ".env", c.content)

			_, err := LoadConfig("auth-service", "auth")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

// TestLoadConfig_UnreadableEnvFile proves a .env that exists but cannot be read (here a
// directory) is an error, not skipped like a missing file.
func TestLoadConfig_UnreadableEnvFile(t *testing.T) {
	dir := isolateConfig(t)
	if err := os.Mkdir(filepath.Join(dir, ".env"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, dir, "services/auth-service/.env", "HTTP_PORT=3001\n")

	if _, err := LoadConfig("auth-service", "auth"); err == nil {
		t.Fatal("expected an error for an unreadable .env")
	}
}

// TestGetEnv proves the fallback is used for an unset or empty variable only.
func TestGetEnv(t *testing.T) {
	isolateConfig(t)
	if got := getEnv("APP_VERSION", "dev"); got != "dev" {
		t.Fatalf("unset: %q", got)
	}
	t.Setenv("APP_VERSION", "")
	if got := getEnv("APP_VERSION", "dev"); got != "dev" {
		t.Fatalf("empty: %q", got)
	}
	t.Setenv("APP_VERSION", "2.0")
	if got := getEnv("APP_VERSION", "dev"); got != "2.0" {
		t.Fatalf("set: %q", got)
	}
}
