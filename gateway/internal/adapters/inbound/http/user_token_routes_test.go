/**
##
## OverDrive 2026
## All Technical rights reserved
##
## user_token_routes_test.go - Tests the per-route choice between the gateway token and a user JWT.
##
*/

package httpinbound

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"overdrive/gateway/internal/adapters/outbound/auth"
	"overdrive/gateway/internal/core/domain"
	"overdrive/gateway/internal/core/usecases"
)

// userJWTSecret is long enough for authjwt.ValidateSecret (32 bytes minimum).
const userJWTSecret = "gateway-test-secret-long-enough-for-hs256"

// gatewayTestToken is the static gateway token of the test handlers.
const gatewayTestToken = "static-gateway-token"

// signTestJWT signs an access token for user-1 valid for ttl (negative = expired). An empty role
// leaves the claim out, like the tokens issued before roles existed.
func signTestJWT(t *testing.T, role string, ttl time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(ttl).Unix()}
	if role != "" {
		claims["role"] = role
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(userJWTSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return token
}

// newTestHandler builds the real gateway handler with the real token validators.
func newTestHandler(t *testing.T, routes []domain.Route) (http.Handler, error) {
	t.Helper()
	userValidator, err := auth.NewUserTokenValidator(userJWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(
		routes, map[string][]*url.URL{}, 1000, 1000, nil,
		usecases.NewCheckHealthUseCase(),
		usecases.NewAuthorizeUserRequestUseCase(userValidator),
		usecases.NewAuthorizeAdminRequestUseCase(auth.NewStaticTokenValidator(gatewayTestToken), userValidator),
	)
}

// TestNewHandler_RouteAccess proves each access policy end to end through the router: public
// routes need no token; user routes accept any role's JWT and refuse the gateway token; admin
// routes accept an admin JWT or the gateway token and answer 403 to a user JWT. Public auth
// routes are an allow-list under a user-only parent, so the sessions, the upstream /health and
// any path dodging the allow-list (encoded slash) fail closed with 401 instead of reaching the
// service unauthenticated. The upstream receives the caller's Authorization header unchanged so
// it can re-verify the JWT itself.
func TestNewHandler_RouteAccess(t *testing.T) {
	var gotAuthorization, gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)
	targets := []*url.URL{upstreamURL}

	handler, err := newTestHandler(t, []domain.Route{
		{Prefix: "/v1/auth", Targets: targets, Access: domain.AccessUser},
		{Prefix: "/v1/auth/login", UpstreamPrefix: "/login", Targets: targets, Access: domain.AccessPublic},
		{Prefix: "/v1/auth/oauth", UpstreamPrefix: "/oauth", Targets: targets, Access: domain.AccessPublic},
		{Prefix: "/v1/presets", UpstreamPrefix: "/presets", Targets: targets, Access: domain.AccessUser},
		{Prefix: "/v1/race-data", Targets: targets, Access: domain.AccessUser},
		{Prefix: "/v1/ingestion", Targets: targets, Access: domain.AccessAdmin},
	})
	if err != nil {
		t.Fatal(err)
	}
	userJWT := "Bearer " + signTestJWT(t, "user", time.Minute)
	legacyJWT := "Bearer " + signTestJWT(t, "", time.Minute)
	adminJWT := "Bearer " + signTestJWT(t, "admin", time.Minute)
	static := "Bearer " + gatewayTestToken

	cases := []struct {
		name, method, path, authorization string
		want                              int
		wantUpstream                      string
	}{
		{"login without token", http.MethodPost, "/v1/auth/login", "", http.StatusOK, "/login"},
		{"oauth providers without token", http.MethodGet, "/v1/auth/oauth/providers", "", http.StatusOK, "/oauth/providers"},
		{"sessions without token", http.MethodGet, "/v1/auth/sessions", "", http.StatusUnauthorized, ""},
		{"encoded slash to sessions", http.MethodDelete, "/v1/auth/sessions%2Fs-1", "", http.StatusUnauthorized, ""},
		{"encoded slash after login", http.MethodGet, "/v1/auth/login%2F..%2Fsessions", "", http.StatusUnauthorized, ""},
		{"auth service health", http.MethodGet, "/v1/auth/health", "", http.StatusUnauthorized, ""},
		{"sessions with static token", http.MethodGet, "/v1/auth/sessions", static, http.StatusUnauthorized, ""},
		{"sessions with user JWT", http.MethodGet, "/v1/auth/sessions", userJWT, http.StatusOK, "/sessions"},
		{"one session with user JWT", http.MethodDelete, "/v1/auth/sessions/s-1", userJWT, http.StatusOK, "/sessions/s-1"},
		{"presets with user JWT", http.MethodGet, "/v1/presets/abc", userJWT, http.StatusOK, "/presets/abc"},
		{"presets with static token", http.MethodGet, "/v1/presets/abc", static, http.StatusUnauthorized, ""},
		{"race-data without token", http.MethodGet, "/v1/race-data/championships", "", http.StatusUnauthorized, ""},
		{"race-data with static token", http.MethodGet, "/v1/race-data/championships", static, http.StatusUnauthorized, ""},
		{"race-data with user JWT", http.MethodGet, "/v1/race-data/championships", userJWT, http.StatusOK, "/championships"},
		{"race-data with role-less JWT", http.MethodGet, "/v1/race-data/championships", legacyJWT, http.StatusOK, "/championships"},
		{"race-data with admin JWT", http.MethodGet, "/v1/race-data/championships", adminJWT, http.StatusOK, "/championships"},
		{"ingestion without token", http.MethodPost, "/v1/ingestion/providers/openf1/ingestions", "", http.StatusUnauthorized, ""},
		{"ingestion with user JWT", http.MethodPost, "/v1/ingestion/providers/openf1/ingestions", userJWT, http.StatusForbidden, ""},
		{"ingestion with role-less JWT", http.MethodPost, "/v1/ingestion/providers/openf1/ingestions", legacyJWT, http.StatusForbidden, ""},
		{"ingestion with admin JWT", http.MethodPost, "/v1/ingestion/providers/openf1/ingestions", adminJWT, http.StatusOK, "/providers/openf1/ingestions"},
		{"ingestion with static token", http.MethodPost, "/v1/ingestion/providers/openf1/ingestions", static, http.StatusOK, "/providers/openf1/ingestions"},
	}
	for _, tc := range cases {
		gotAuthorization, gotPath = "", ""
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.authorization != "" {
			req.Header.Set("Authorization", tc.authorization)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s: status = %d body=%s, want %d", tc.name, rec.Code, rec.Body.String(), tc.want)
		}
		if gotPath != tc.wantUpstream {
			t.Fatalf("%s: upstream path = %q, want %q", tc.name, gotPath, tc.wantUpstream)
		}
		if tc.wantUpstream != "" && gotAuthorization != tc.authorization {
			t.Fatalf("%s: forwarded Authorization %q, want %q", tc.name, gotAuthorization, tc.authorization)
		}
	}
}

// TestNewHandler_UserTokenRejectionCodes proves each rejection on a user route carries its own
// code - in particular TOKEN_EXPIRED, which tells the client to refresh instead of logging in
// again - and that a user JWT on an admin route is FORBIDDEN, not a 401.
func TestNewHandler_UserTokenRejectionCodes(t *testing.T) {
	upstreamURL, _ := url.Parse("http://127.0.0.1:1")
	handler, err := newTestHandler(t, []domain.Route{
		{Prefix: "/v1/presets", UpstreamPrefix: "/presets", Targets: []*url.URL{upstreamURL}, Access: domain.AccessUser},
		{Prefix: "/v1/ingestion", Targets: []*url.URL{upstreamURL}, Access: domain.AccessAdmin},
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		path, authorization, code string
		status                    int
	}{
		"no token":          {"/v1/presets", "", `"UNAUTHORIZED"`, http.StatusUnauthorized},
		"expired JWT":       {"/v1/presets", "Bearer " + signTestJWT(t, "user", -time.Minute), `"TOKEN_EXPIRED"`, http.StatusUnauthorized},
		"forged token":      {"/v1/presets", "Bearer not-a-jwt", `"INVALID_TOKEN"`, http.StatusUnauthorized},
		"expired admin JWT": {"/v1/ingestion", "Bearer " + signTestJWT(t, "admin", -time.Minute), `"TOKEN_EXPIRED"`, http.StatusUnauthorized},
		"user JWT on admin": {"/v1/ingestion", "Bearer " + signTestJWT(t, "user", time.Minute), `"FORBIDDEN"`, http.StatusForbidden},
		"no token on admin": {"/v1/ingestion", "", `"UNAUTHORIZED"`, http.StatusUnauthorized},
		"bad static token":  {"/v1/ingestion", "Bearer wrong-static-token", `"INVALID_TOKEN"`, http.StatusUnauthorized},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if tc.authorization != "" {
			req.Header.Set("Authorization", tc.authorization)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.code) {
			t.Fatalf("%s: status = %d body=%s, want %d %s", name, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
	}
}

// TestNewHandler_RejectsInvalidRouteTables proves the router fails closed at build time on a
// route without an access policy, on a prefix declared twice and on admin methods outside a user
// route, instead of serving it open or
// panicking inside ServeMux.
func TestNewHandler_RejectsInvalidRouteTables(t *testing.T) {
	upstreamURL, _ := url.Parse("http://127.0.0.1:1")
	targets := []*url.URL{upstreamURL}
	for name, routes := range map[string][]domain.Route{
		"no access policy": {{Prefix: "/v1/race-data", Targets: targets}},
		"duplicate prefix": {
			{Prefix: "/v1/race-data", Targets: targets, Access: domain.AccessUser},
			{Prefix: "/v1/race-data/", Targets: targets, Access: domain.AccessPublic},
		},
		"admin methods on a public route": {
			{Prefix: "/v1/race-data", Targets: targets, Access: domain.AccessPublic, AdminMethods: []string{http.MethodPut}},
		},
	} {
		if _, err := newTestHandler(t, routes); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

// TestNewHandler_AdminMethods proves a user route restricts its admin methods to administrators
// on every path: a feed PUT needs an admin JWT or the gateway token (403 for a user JWT, and
// the edge method gate is only reached by an administrator), an encoded slash does not dodge the
// rule, reads stay open to any user JWT, and the gateway token still cannot read.
func TestNewHandler_AdminMethods(t *testing.T) {
	var gotMethod, gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)
	targets := []*url.URL{upstreamURL}

	adminMethods := []string{http.MethodPut}
	handler, err := newTestHandler(t, []domain.Route{
		{Prefix: "/v1/championship", Targets: targets, Access: domain.AccessUser, AdminMethods: adminMethods},
		{Prefix: "/v1/race-data", Targets: targets, Access: domain.AccessUser, AdminMethods: adminMethods},
	})
	if err != nil {
		t.Fatal(err)
	}
	userJWT := "Bearer " + signTestJWT(t, "user", time.Minute)
	legacyJWT := "Bearer " + signTestJWT(t, "", time.Minute)
	adminJWT := "Bearer " + signTestJWT(t, "admin", time.Minute)
	expiredAdminJWT := "Bearer " + signTestJWT(t, "admin", -time.Minute)
	static := "Bearer " + gatewayTestToken

	const driverPath, sessionPath = "/sessions/s1/drivers/44/broadcast", "/sessions/s1/broadcast"
	for _, prefix := range []string{"/v1/championship", "/v1/race-data"} {
		feedPath := sessionPath
		if prefix == "/v1/race-data" {
			feedPath = driverPath
		}
		cases := []struct {
			name, method, path, authorization string
			want                              int
			code                              string
		}{
			{"PUT without token", http.MethodPut, feedPath, "", http.StatusUnauthorized, "UNAUTHORIZED"},
			{"PUT with user JWT", http.MethodPut, feedPath, userJWT, http.StatusForbidden, "FORBIDDEN"},
			{"PUT with role-less JWT", http.MethodPut, feedPath, legacyJWT, http.StatusForbidden, "FORBIDDEN"},
			{"PUT with expired admin JWT", http.MethodPut, feedPath, expiredAdminJWT, http.StatusUnauthorized, "TOKEN_EXPIRED"},
			{"PUT with wrong token", http.MethodPut, feedPath, "Bearer wrong", http.StatusUnauthorized, "INVALID_TOKEN"},
			{"PUT with encoded slash and user JWT", http.MethodPut, strings.Replace(feedPath, "/s1/", "%2Fs1/", 1), userJWT, http.StatusForbidden, "FORBIDDEN"},
			{"PUT elsewhere with user JWT", http.MethodPut, "/sessions/s1", userJWT, http.StatusForbidden, "FORBIDDEN"},
			{"PUT elsewhere with admin JWT", http.MethodPut, "/sessions/s1", adminJWT, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED"},
			{"PUT with admin JWT", http.MethodPut, feedPath, adminJWT, http.StatusOK, ""},
			{"PUT with gateway token", http.MethodPut, feedPath, static, http.StatusOK, ""},
			{"GET with user JWT", http.MethodGet, feedPath, userJWT, http.StatusOK, ""},
			{"GET with admin JWT", http.MethodGet, feedPath, adminJWT, http.StatusOK, ""},
			{"GET with gateway token", http.MethodGet, feedPath, static, http.StatusUnauthorized, "INVALID_TOKEN"},
			{"DELETE with user JWT", http.MethodDelete, feedPath, userJWT, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED"},
		}
		for _, tc := range cases {
			gotMethod, gotPath = "", ""
			req := httptest.NewRequest(tc.method, prefix+tc.path, nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("%s %s: status = %d body=%s, want %d %s", prefix, tc.name, rec.Code, rec.Body.String(), tc.want, tc.code)
			}
			if tc.want == http.StatusOK && (gotMethod != tc.method || gotPath != tc.path) {
				t.Fatalf("%s %s: upstream got %s %s, want %s %s", prefix, tc.name, gotMethod, gotPath, tc.method, tc.path)
			}
			if tc.want != http.StatusOK && gotPath != "" {
				t.Fatalf("%s %s: upstream reached with %s %s", prefix, tc.name, gotMethod, gotPath)
			}
		}
	}
}
