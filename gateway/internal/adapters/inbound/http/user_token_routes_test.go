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
	"overdrive/gateway/internal/core/usecases"
)

// TestNewHandler_UserTokenRoutes proves /presets is authorised with a user JWT (and rejects the
// static gateway token), while other prefixes keep requiring the static token (and reject a
// user JWT). The upstream receives the caller's Authorization header unchanged so it can
// re-verify the JWT itself. On /presets, each rejection carries its own 401 code — in
// particular TOKEN_EXPIRED, which tells the client to refresh instead of logging in again.
func TestNewHandler_UserTokenRoutes(t *testing.T) {
	var gotAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)

	userValidator, err := auth.NewUserTokenValidator("jwt-secret")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(
		map[string][]*url.URL{"/presets": {upstreamURL}, "/v1/race-data": {upstreamURL}},
		map[string][]*url.URL{}, 1000, 1000,
		usecases.NewCheckHealthUseCase(),
		usecases.NewAuthorizeRequestUseCase(auth.NewStaticTokenValidator("static-token")),
		usecases.NewAuthorizeUserRequestUseCase(userValidator),
	)
	if err != nil {
		t.Fatal(err)
	}
	userJWT, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte("jwt-secret"))

	serve := func(path, authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	call := func(path, authorization string) int { return serve(path, authorization).Code }

	if code := call("/presets", "Bearer "+userJWT); code != http.StatusOK || gotAuthorization != "Bearer "+userJWT {
		t.Fatalf("/presets with user JWT: status = %d, forwarded Authorization %q", code, gotAuthorization)
	}
	for name, tc := range map[string]struct {
		path, authorization string
		want                int
	}{
		"presets with static token": {"/presets/abc", "Bearer static-token", http.StatusUnauthorized},
		"presets without token":     {"/presets", "", http.StatusUnauthorized},
		"race-data with static":     {"/v1/race-data/championships", "Bearer static-token", http.StatusOK},
		"race-data with user JWT":   {"/v1/race-data/championships", "Bearer " + userJWT, http.StatusUnauthorized},
	} {
		if code := call(tc.path, tc.authorization); code != tc.want {
			t.Fatalf("%s: status = %d, want %d", name, code, tc.want)
		}
	}

	expiredJWT, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(-time.Minute).Unix()}).SignedString([]byte("jwt-secret"))
	for name, tc := range map[string]struct{ authorization, code string }{
		"no token":     {"", `"UNAUTHORIZED"`},
		"expired JWT":  {"Bearer " + expiredJWT, `"TOKEN_EXPIRED"`},
		"forged token": {"Bearer static-token", `"INVALID_TOKEN"`},
	} {
		rec := serve("/presets", tc.authorization)
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), tc.code) {
			t.Fatalf("/presets %s: status = %d body=%s, want 401 %s", name, rec.Code, rec.Body.String(), tc.code)
		}
	}
}
