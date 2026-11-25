/**
##
## OverDrive 2026
## All Technical rights reserved
##
## bootstrap_test.go - Unit tests for the wiring of the gateway runtime.
##
*/

package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"overdrive/gateway/internal/config"
)

const (
	testGatewayToken = "gateway-token-0123456789abcdef-0123456789abcdef"
	testJWTSecret    = "jwt-secret-0123456789abcdef-0123456789abcdef"
)

// loadTestConfig loads a real gateway configuration (through config.Load) whose every
// upstream is the given URL, from a gateway/.env in a temporary working directory.
func loadTestConfig(t *testing.T, upstream string, jwtSecret string) config.Config {
	t.Helper()
	env := map[string]string{
		"HTTP_PORT":                 "3000",
		"GATEWAY_AUTH_TOKEN":        testGatewayToken,
		"AUTH_JWT_SECRET":           jwtSecret,
		"GATEWAY_RATE_LIMIT_RPS":    "1000",
		"GATEWAY_RATE_LIMIT_BURST":  "1000",
		"GATEWAY_TRUSTED_PROXIES":   "",
		"AUTH_SERVICE_URLS":         upstream,
		"USER_DATA_SERVICE_URLS":    upstream,
		"CHAMPIONSHIP_SERVICE_URLS": upstream,
		"RACE_DATA_SERVICE_URLS":    upstream,
		"INGESTION_SERVICE_URLS":    upstream,
	}
	var content strings.Builder
	for key, value := range env {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		content.WriteString(key + "=" + value + "\n")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "gateway"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gateway", ".env"), []byte(content.String()), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Chdir(dir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

// signJWT returns an HS256 token for the user with the given role ("" = no role claim).
func signJWT(t *testing.T, secret string, role string) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(time.Minute).Unix()}
	if role != "" {
		claims["role"] = role
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return token
}

// TestBuild_RefusesWeakJWTSecret proves the runtime is not built when AUTH_JWT_SECRET is too
// short: config.Load only checks it is present, the user token validator enforces its strength.
func TestBuild_RefusesWeakJWTSecret(t *testing.T) {
	cfg := loadTestConfig(t, "http://127.0.0.1:1", "short-secret")

	if _, err := Build(cfg); err == nil {
		t.Fatal("expected Build to refuse a short AUTH_JWT_SECRET")
	}
}

// TestBuild_WiresAccessRules proves the validators are wired to the right routes: public auth
// routes need nothing, user routes take any valid user JWT but not the gateway token, admin
// routes take an admin JWT or the gateway token but not a user JWT, and a JWT signed with
// another secret is refused.
func TestBuild_WiresAccessRules(t *testing.T) {
	var mu sync.Mutex
	var reached []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		reached = append(reached, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	runtime, err := Build(loadTestConfig(t, upstream.URL, testJWTSecret))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	userJWT := signJWT(t, testJWTSecret, "")
	adminJWT := signJWT(t, testJWTSecret, "admin")
	foreignJWT := signJWT(t, "another-secret-0123456789abcdef-0123456789", "admin")

	cases := []struct {
		name, method, path, token string
		status                    int
		upstream                  string
	}{
		{"public login", http.MethodPost, "/v1/auth/login", "", http.StatusOK, "POST /login"},
		{"user route without token", http.MethodGet, "/v1/presets", "", http.StatusUnauthorized, ""},
		{"user route with user JWT", http.MethodGet, "/v1/presets", userJWT, http.StatusOK, "GET /presets"},
		{"user route with gateway token", http.MethodGet, "/v1/presets", testGatewayToken, http.StatusUnauthorized, ""},
		{"user route with foreign JWT", http.MethodGet, "/v1/presets", foreignJWT, http.StatusUnauthorized, ""},
		{"admin route with user JWT", http.MethodPost, "/v1/ingestion/jobs", userJWT, http.StatusForbidden, ""},
		{"admin route with admin JWT", http.MethodPost, "/v1/ingestion/jobs", adminJWT, http.StatusOK, "POST /jobs"},
		{"admin route with gateway token", http.MethodPost, "/v1/ingestion/jobs", testGatewayToken, http.StatusOK, "POST /jobs"},
		{"admin route without token", http.MethodPost, "/v1/ingestion/jobs", "", http.StatusUnauthorized, ""},
		{"session feeds with user JWT", http.MethodPut, "/v1/championship/sessions/s1/broadcast", userJWT, http.StatusForbidden, ""},
		{"session feeds with admin JWT", http.MethodPut, "/v1/championship/sessions/s1/broadcast", adminJWT, http.StatusOK, "PUT /sessions/s1/broadcast"},
		{"driver feeds with user JWT", http.MethodPut, "/v1/race-data/sessions/s1/drivers/44/broadcast", userJWT, http.StatusForbidden, ""},
		{"driver feeds with gateway token", http.MethodPut, "/v1/race-data/sessions/s1/drivers/44/broadcast", testGatewayToken, http.StatusOK, "PUT /sessions/s1/drivers/44/broadcast"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mu.Lock()
			reached = nil
			mu.Unlock()
			req := httptest.NewRequest(c.method, c.path, nil)
			req.RemoteAddr = "192.0.2.1:1234"
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}
			rec := httptest.NewRecorder()
			runtime.Handler.ServeHTTP(rec, req)
			mu.Lock()
			defer mu.Unlock()

			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
			if c.upstream == "" && len(reached) != 0 {
				t.Fatalf("upstream reached %v, want nothing", reached)
			}
			if c.upstream != "" && (len(reached) != 1 || reached[0] != c.upstream) {
				t.Fatalf("upstream reached %v, want %q", reached, c.upstream)
			}
		})
	}
}
