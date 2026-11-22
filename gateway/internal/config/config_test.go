/**
##
## OverDrive 2026
## All Technical rights reserved
##
## config_test.go - Unit tests for the gateway configuration loader.
##
*/

package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// testGatewayToken and testJWTSecret are 32 bytes or more, like the real secrets must be.
const (
	testGatewayToken = "gateway-token-0123456789abcdef-0123456789abcdef"
	testJWTSecret    = "jwt-secret-0123456789abcdef-0123456789abcdef"
)

// validGatewayEnv is a complete configuration Load accepts.
func validGatewayEnv() map[string]string {
	return map[string]string{
		"HTTP_PORT":                 "3000",
		"GATEWAY_AUTH_TOKEN":        testGatewayToken,
		"AUTH_JWT_SECRET":           testJWTSecret,
		"GATEWAY_RATE_LIMIT_RPS":    "20",
		"GATEWAY_RATE_LIMIT_BURST":  "40",
		"GATEWAY_TRUSTED_PROXIES":   "10.0.0.1, 192.168.0.0/16",
		"AUTH_SERVICE_URLS":         "http://auth:3001",
		"USER_DATA_SERVICE_URLS":    "http://user-data:3002",
		"CHAMPIONSHIP_SERVICE_URLS": "http://championship:3003",
		"RACE_DATA_SERVICE_URLS":    "http://race-data-1:3004, http://race-data-2:3004",
		"INGESTION_SERVICE_URLS":    "http://ingestion:3005",
	}
}

// isolateGatewayEnv runs the test in an empty directory with every gateway variable unset,
// restoring them afterwards (the loader sets them with os.Setenv).
func isolateGatewayEnv(t *testing.T) string {
	t.Helper()
	for key := range validGatewayEnv() {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

// writeGatewayEnv writes env as dir/gateway/.env, skipping the keys listed in omit.
func writeGatewayEnv(t *testing.T, dir string, env map[string]string, omit ...string) {
	t.Helper()
	skipped := map[string]bool{}
	for _, key := range omit {
		skipped[key] = true
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		if !skipped[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var content strings.Builder
	for _, key := range keys {
		content.WriteString(key + "=" + env[key] + "\n")
	}
	if err := os.MkdirAll(filepath.Join(dir, "gateway"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gateway", ".env"), []byte(content.String()), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
}

// TestLoad_ValidConfig proves a complete gateway/.env loads into the config: values, trusted
// proxies, every upstream list (several targets allowed) and the derived route tables.
func TestLoad_ValidConfig(t *testing.T) {
	dir := isolateGatewayEnv(t)
	writeGatewayEnv(t, dir, validGatewayEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPPort != "3000" || cfg.AuthToken != testGatewayToken || cfg.UserJWTSecret != testJWTSecret {
		t.Fatalf("unexpected values: port=%q token=%q secret=%q", cfg.HTTPPort, cfg.AuthToken, cfg.UserJWTSecret)
	}
	if cfg.RateLimitRPS != 20 || cfg.RateLimitBurst != 40 {
		t.Fatalf("rate limit = %v/%v", cfg.RateLimitRPS, cfg.RateLimitBurst)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0].String() != "10.0.0.1/32" || cfg.TrustedProxies[1].String() != "192.168.0.0/16" {
		t.Fatalf("trusted proxies = %v", cfg.TrustedProxies)
	}
	if len(cfg.Routes) == 0 {
		t.Fatal("no proxied routes built")
	}
	wantHealth := map[string]string{
		"/health/auth":         "http://auth:3001",
		"/health/user-data":    "http://user-data:3002",
		"/health/championship": "http://championship:3003",
		"/health/race-data":    "http://race-data-1:3004",
		"/health/ingestion":    "http://ingestion:3005",
	}
	for path, first := range wantHealth {
		targets := cfg.ServiceHealthRoutes[path]
		if len(targets) == 0 || targets[0].String() != first {
			t.Errorf("health route %s = %v, want first target %s", path, targets, first)
		}
	}
	if got := len(cfg.ServiceHealthRoutes["/health/race-data"]); got != 2 {
		t.Errorf("race-data targets = %d, want 2", got)
	}
}

// TestLoad_RequiredVariables proves each required variable stops the start when missing, with
// an error naming it, so a regression on GATEWAY_AUTH_TOKEN or AUTH_JWT_SECRET is caught.
func TestLoad_RequiredVariables(t *testing.T) {
	required := []string{
		"HTTP_PORT", "GATEWAY_AUTH_TOKEN", "AUTH_JWT_SECRET", "GATEWAY_RATE_LIMIT_RPS", "GATEWAY_RATE_LIMIT_BURST",
		"AUTH_SERVICE_URLS", "USER_DATA_SERVICE_URLS", "CHAMPIONSHIP_SERVICE_URLS", "RACE_DATA_SERVICE_URLS", "INGESTION_SERVICE_URLS",
	}
	for _, key := range required {
		t.Run(key, func(t *testing.T) {
			dir := isolateGatewayEnv(t)
			writeGatewayEnv(t, dir, validGatewayEnv(), key)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), key+" is required") {
				t.Fatalf("err = %v, want the %s requirement", err, key)
			}
		})
	}
}

// TestLoad_EmptyRequiredVariable proves an empty value counts as missing.
func TestLoad_EmptyRequiredVariable(t *testing.T) {
	dir := isolateGatewayEnv(t)
	env := validGatewayEnv()
	env["AUTH_JWT_SECRET"] = ""
	writeGatewayEnv(t, dir, env)

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AUTH_JWT_SECRET is required") {
		t.Fatalf("err = %v, want the AUTH_JWT_SECRET requirement", err)
	}
}

// TestLoad_TrustedProxiesOptional proves GATEWAY_TRUSTED_PROXIES may be absent: no proxy is
// trusted and the TCP peer identifies the client.
func TestLoad_TrustedProxiesOptional(t *testing.T) {
	dir := isolateGatewayEnv(t)
	writeGatewayEnv(t, dir, validGatewayEnv(), "GATEWAY_TRUSTED_PROXIES")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("trusted proxies = %v, want none", cfg.TrustedProxies)
	}
}

// TestLoad_ShortGatewayToken proves a GATEWAY_AUTH_TOKEN under 32 bytes stops the start,
// also when padding with spaces brings the raw value over the limit.
func TestLoad_ShortGatewayToken(t *testing.T) {
	for name, token := range map[string]string{"short": "too-short", "padded": "  too-short-but-padded                 "} {
		t.Run(name, func(t *testing.T) {
			dir := isolateGatewayEnv(t)
			writeGatewayEnv(t, dir, validGatewayEnv(), "GATEWAY_AUTH_TOKEN")
			t.Setenv("GATEWAY_AUTH_TOKEN", token)

			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "GATEWAY_AUTH_TOKEN must be at least 32 bytes") {
				t.Fatalf("err = %v, want the token length error", err)
			}
		})
	}
}

// TestLoad_GatewayTokenTrimmed proves the configured token is trimmed, like the presented one
// is before comparison.
func TestLoad_GatewayTokenTrimmed(t *testing.T) {
	dir := isolateGatewayEnv(t)
	writeGatewayEnv(t, dir, validGatewayEnv(), "GATEWAY_AUTH_TOKEN")
	t.Setenv("GATEWAY_AUTH_TOKEN", "  "+testGatewayToken+"\t")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AuthToken != testGatewayToken {
		t.Fatalf("AuthToken = %q, want it trimmed", cfg.AuthToken)
	}
}

// TestLoad_InvalidValues proves each malformed value stops the start with an error naming the
// variable: non-numeric or non-positive rate limits, unparsable proxies, upstream lists that
// are not absolute URLs or are empty once split.
func TestLoad_InvalidValues(t *testing.T) {
	cases := []struct {
		key, value, want string
	}{
		{"GATEWAY_RATE_LIMIT_RPS", "fast", "GATEWAY_RATE_LIMIT_RPS must be a number"},
		{"GATEWAY_RATE_LIMIT_RPS", "0", "GATEWAY_RATE_LIMIT_RPS must be greater than 0"},
		{"GATEWAY_RATE_LIMIT_BURST", "-5", "GATEWAY_RATE_LIMIT_BURST must be greater than 0"},
		{"GATEWAY_TRUSTED_PROXIES", "10.0.0.1, not-an-ip", `GATEWAY_TRUSTED_PROXIES: invalid entry "not-an-ip/128"`},
		{"AUTH_SERVICE_URLS", "auth:3001", "AUTH_SERVICE_URLS: invalid target"},
		{"USER_DATA_SERVICE_URLS", " , ", "USER_DATA_SERVICE_URLS: at least one target is required"},
		{"CHAMPIONSHIP_SERVICE_URLS", "http://", "CHAMPIONSHIP_SERVICE_URLS: invalid target"},
		{"RACE_DATA_SERVICE_URLS", "http://ok:3004,/relative", "RACE_DATA_SERVICE_URLS: invalid target"},
		{"INGESTION_SERVICE_URLS", "http://bad host", "INGESTION_SERVICE_URLS:"},
	}
	for _, c := range cases {
		t.Run(c.key+"="+c.value, func(t *testing.T) {
			dir := isolateGatewayEnv(t)
			env := validGatewayEnv()
			env[c.key] = c.value
			writeGatewayEnv(t, dir, env)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

// TestLoad_EnvironmentWinsOverFile proves a variable set by Compose or the shell is not
// replaced by gateway/.env.
func TestLoad_EnvironmentWinsOverFile(t *testing.T) {
	dir := isolateGatewayEnv(t)
	writeGatewayEnv(t, dir, validGatewayEnv())
	t.Setenv("HTTP_PORT", "8080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Fatalf("HTTP_PORT = %q, want the environment value", cfg.HTTPPort)
	}
}

// TestLoad_FromGatewayDirectory proves the gateway also starts from its own directory, reading
// ./.env when the working directory is named gateway.
func TestLoad_FromGatewayDirectory(t *testing.T) {
	dir := isolateGatewayEnv(t)
	writeGatewayEnv(t, dir, validGatewayEnv())
	t.Chdir(filepath.Join(dir, "gateway"))

	if _, err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

// TestLoad_NoEnvFile proves the gateway refuses to start without gateway/.env, even when the
// environment holds every variable.
func TestLoad_NoEnvFile(t *testing.T) {
	isolateGatewayEnv(t)
	for key, value := range validGatewayEnv() {
		t.Setenv(key, value)
	}

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "unable to find gateway .env") {
		t.Fatalf("err = %v, want the missing .env error", err)
	}
}

// TestLoad_InvalidEnvLines proves a malformed gateway/.env line stops the start instead of
// being ignored.
func TestLoad_InvalidEnvLines(t *testing.T) {
	for name, line := range map[string]string{"no equals": "NOT_A_PAIR", "empty key": "=value"} {
		t.Run(name, func(t *testing.T) {
			dir := isolateGatewayEnv(t)
			writeGatewayEnv(t, dir, validGatewayEnv())
			path := filepath.Join(dir, "gateway", ".env")
			content, err := os.ReadFile(path) // #nosec G304 -- test file under t.TempDir
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if err := os.WriteFile(path, append(content, []byte(line+"\n")...), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}

			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "invalid") {
				t.Fatalf("err = %v, want an invalid .env line error", err)
			}
		})
	}
}

// TestLoad_UnreadableEnvFile proves a gateway/.env that exists but cannot be read (here a
// directory) is an error, not treated as missing.
func TestLoad_UnreadableEnvFile(t *testing.T) {
	dir := isolateGatewayEnv(t)
	if err := os.MkdirAll(filepath.Join(dir, "gateway", ".env"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := Load()
	if err == nil || strings.Contains(err.Error(), "unable to find") {
		t.Fatalf("err = %v, want a read error", err)
	}
}
