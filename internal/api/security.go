/**
##
## OverDrive 2026
## All Technical rights reserved
##
## security.go - Security middleware for CORS, rate limiting, JWT parsing, and request hardening.
##
*/

package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"overdrive/internal/config"
	"strings"
	"sync"
	"time"
)

type authContextKey string

const authKey authContextKey = "auth_context"

type AuthContext struct {
	Subject   string
	UserID    string
	Roles     []string
	ExpiresAt time.Time
	Issuer    string
	Audience  []string
}

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

type ipRateLimiter struct {
	limit   int
	window  time.Duration
	mu      sync.Mutex
	entries map[string]rateLimitEntry
}

// newIPRateLimiter builds the in-memory limiter used to guard the public HTTP surface.
func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &ipRateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]rateLimitEntry),
	}
}

// Allow returns whether the client key can pass the current rate-limit window.
func (l *ipRateLimiter) Allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for ip, entry := range l.entries {
		if now.After(entry.resetAt) {
			delete(l.entries, ip)
		}
	}

	entry, ok := l.entries[key]
	if !ok || now.After(entry.resetAt) {
		l.entries[key] = rateLimitEntry{
			count:   1,
			resetAt: now.Add(l.window),
		}
		return true, 0
	}

	if entry.count >= l.limit {
		return false, time.Until(entry.resetAt)
	}

	entry.count++
	l.entries[key] = entry
	return true, 0
}

// normalizeSecurityConfig ensures security middleware always has sane defensive defaults.
func normalizeSecurityConfig(cfg config.Config) config.Config {
	if cfg.RateLimitRequests <= 0 {
		cfg.RateLimitRequests = 60
	}
	if cfg.RateLimitWindow <= 0 {
		cfg.RateLimitWindow = time.Minute
	}
	if cfg.MaxPathLength <= 0 {
		cfg.MaxPathLength = 512
	}
	if cfg.MaxQueryLength <= 0 {
		cfg.MaxQueryLength = 2048
	}
	return cfg
}

// withSecurityHeaders adds browser-facing hardening headers on every response.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// withRequestValidation rejects obviously abusive request targets before handler execution.
func withRequestValidation(logger *slog.Logger, cfg config.Config) func(http.Handler) http.Handler {
	cfg = normalizeSecurityConfig(cfg)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.URL.Path) > cfg.MaxPathLength {
				logSuspicious(logger, r, "request_rejected", "path_too_long", slog.Int("path_length", len(r.URL.Path)))
				writeError(w, http.StatusRequestURITooLong, "request path too long")
				return
			}
			if len(r.URL.RawQuery) > cfg.MaxQueryLength {
				logSuspicious(logger, r, "request_rejected", "query_too_long", slog.Int("query_length", len(r.URL.RawQuery)))
				writeError(w, http.StatusRequestURITooLong, "request query too long")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// withCORS enforces the configured browser origin allowlist and answers valid preflights.
func withCORS(logger *slog.Logger, cfg config.Config) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(cfg.CORSAllowedOrigins))
	for _, origin := range cfg.CORSAllowedOrigins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Add("Vary", "Origin")
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")

			if _, ok := allowed[origin]; !ok {
				logSuspicious(logger, r, "cors_rejected", "origin_not_allowed", slog.String("origin", origin))
				writeError(w, http.StatusForbidden, "origin not allowed")
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "600")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// withRateLimit applies a simple in-memory per-IP rate limit for the current process.
func withRateLimit(logger *slog.Logger, cfg config.Config) func(http.Handler) http.Handler {
	cfg = normalizeSecurityConfig(cfg)
	limiter := newIPRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow)
	trustedProxies := parseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := clientIPFromRequest(r, trustedProxies)
			if clientIP == "" {
				clientIP = "unknown"
			}

			allowed, retryAfter := limiter.Allow(clientIP, time.Now())
			if !allowed {
				logSuspicious(logger, r, "rate_limited", "too_many_requests", slog.Int64("retry_after_seconds", int64(retryAfter.Seconds())+1))
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int64(retryAfter.Seconds())+1))
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// withOptionalAuth validates a presented JWT without making the current public API mandatory-auth.
func withOptionalAuth(logger *slog.Logger, cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerTokenFromRequest(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			if strings.TrimSpace(cfg.JWTSecret) == "" {
				logSuspicious(logger, r, "auth_rejected", "jwt_secret_not_configured")
				writeError(w, http.StatusUnauthorized, "authentication unavailable")
				return
			}

			authCtx, err := validateJWT(token, cfg)
			if err != nil {
				logSuspicious(logger, r, "auth_rejected", "invalid_token", slog.String("error", err.Error()))
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), authKey, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requireRoles is kept as the RBAC hook for future protected routes.
// It is intentionally unused for now because the current API surface remains public.
//
//nolint:unused
func requireRoles(roles ...string) func(http.Handler) http.Handler {
	required := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		required[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx, ok := authContextFromRequest(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			for _, role := range authCtx.Roles {
				if _, allowed := required[role]; allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeError(w, http.StatusForbidden, "insufficient permissions")
		})
	}
}

// authContextFromRequest returns validated auth data attached by withOptionalAuth.
func authContextFromRequest(r *http.Request) (AuthContext, bool) {
	ctx, ok := r.Context().Value(authKey).(AuthContext)
	return ctx, ok
}

// logSuspicious emits one structured warning event for blocked or suspicious requests.
func logSuspicious(logger *slog.Logger, r *http.Request, eventType string, reason string, attrs ...slog.Attr) {
	if logger == nil {
		return
	}

	requestAttrs := []any{
		slog.Group("request",
			"id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"client_ip", clientIPFromRemoteAddr(r.RemoteAddr),
		),
		"type", "suspicious_activity",
		"event", eventType,
		"reason", reason,
	}
	for _, attr := range attrs {
		requestAttrs = append(requestAttrs, attr)
	}
	logger.Warn("Suspicious request blocked", requestAttrs...)
}

// bearerTokenFromRequest extracts a token from Authorization or WS-style query param input.
func bearerTokenFromRequest(r *http.Request) (string, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token := strings.TrimSpace(authHeader[7:])
		return token, token != ""
	}

	if isWebSocketUpgrade(r) {
		token := strings.TrimSpace(r.URL.Query().Get("access_token"))
		return token, token != ""
	}

	return "", false
}

// isWebSocketUpgrade detects a WebSocket upgrade handshake.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Sub    string          `json:"sub"`
	UserID string          `json:"user_id"`
	Exp    int64           `json:"exp"`
	Nbf    int64           `json:"nbf"`
	Iss    string          `json:"iss"`
	Aud    json.RawMessage `json:"aud"`
	Roles  []string        `json:"roles"`
	Role   string          `json:"role"`
}

// validateJWT verifies the token signature and security claims against runtime config.
func validateJWT(token string, cfg config.Config) (AuthContext, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AuthContext{}, fmt.Errorf("malformed token")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedMAC := hmac.New(sha256.New, []byte(cfg.JWTSecret))
	_, _ = expectedMAC.Write([]byte(signingInput))
	expectedSignature := expectedMAC.Sum(nil)

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return AuthContext{}, fmt.Errorf("decode signature: %w", err)
	}
	if !hmac.Equal(signature, expectedSignature) {
		return AuthContext{}, fmt.Errorf("invalid signature")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AuthContext{}, fmt.Errorf("decode header: %w", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return AuthContext{}, fmt.Errorf("decode header json: %w", err)
	}
	if !strings.EqualFold(header.Alg, "HS256") {
		return AuthContext{}, fmt.Errorf("unsupported alg")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AuthContext{}, fmt.Errorf("decode claims: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return AuthContext{}, fmt.Errorf("decode claims json: %w", err)
	}
	if claims.Exp <= 0 {
		return AuthContext{}, fmt.Errorf("missing exp")
	}

	now := time.Now().UTC()
	expiresAt := time.Unix(claims.Exp, 0).UTC()
	if !expiresAt.After(now) {
		return AuthContext{}, fmt.Errorf("token expired")
	}
	if claims.Nbf > 0 && time.Unix(claims.Nbf, 0).UTC().After(now) {
		return AuthContext{}, fmt.Errorf("token not active yet")
	}
	if cfg.JWTIssuer != "" && claims.Iss != cfg.JWTIssuer {
		return AuthContext{}, fmt.Errorf("invalid issuer")
	}

	audience := jwtAudience(claims.Aud)
	if cfg.JWTAudience != "" && !containsString(audience, cfg.JWTAudience) {
		return AuthContext{}, fmt.Errorf("invalid audience")
	}

	roles := append([]string{}, claims.Roles...)
	if claims.Role != "" && !containsString(roles, claims.Role) {
		roles = append(roles, claims.Role)
	}

	return AuthContext{
		Subject:   claims.Sub,
		UserID:    claims.UserID,
		Roles:     roles,
		ExpiresAt: expiresAt,
		Issuer:    claims.Iss,
		Audience:  audience,
	}, nil
}

// jwtAudience normalizes the JWT audience claim whether it is encoded as a string or array.
func jwtAudience(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if single == "" {
			return nil
		}
		return []string{single}
	}

	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err == nil {
		return multiple
	}

	return nil
}

// containsString reports whether a string slice contains the requested value.
func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func parseTrustedProxyCIDRs(raw []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if strings.Contains(item, "/") {
			_, network, err := net.ParseCIDR(item)
			if err == nil {
				out = append(out, network)
			}
			continue
		}

		ip := net.ParseIP(item)
		if ip == nil {
			continue
		}
		maskSize := 32
		if ip.To4() == nil {
			maskSize = 128
		}
		out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(maskSize, maskSize)})
	}
	return out
}

func clientIPFromRequest(r *http.Request, trustedProxies []*net.IPNet) string {
	remoteIP := clientIPFromRemoteAddr(r.RemoteAddr)
	if remoteIP == "" {
		return ""
	}
	if len(trustedProxies) == 0 {
		return remoteIP
	}

	parsedRemote := net.ParseIP(remoteIP)
	if parsedRemote == nil || !ipInNetworks(parsedRemote, trustedProxies) {
		return remoteIP
	}

	if forwarded := firstForwardedIP(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return forwarded
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if parsed := net.ParseIP(realIP); parsed != nil {
			return parsed.String()
		}
	}

	return remoteIP
}

func ipInNetworks(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func firstForwardedIP(header string) string {
	if strings.TrimSpace(header) == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if parsed := net.ParseIP(part); parsed != nil {
			return parsed.String()
		}
	}
	return ""
}
