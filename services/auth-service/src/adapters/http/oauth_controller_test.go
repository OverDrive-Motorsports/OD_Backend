/**
##
## OverDrive 2026
## All Technical rights reserved
##
## oauth_controller_test.go - Tests for the OAuth handlers and routes of AuthController.
##
*/

package httpadapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// newOAuthTestRouter goes through NewRouter so route patterns (and ServeMux conflicts, which
// panic at registration) are exercised too.
func newOAuthTestRouter(usecase *fakeAuthUseCase) http.Handler {
	return NewRouter(NewHealthController(nil), NewSessionController(nil), NewAuthController(usecase), usecase)
}

func TestOAuthRoutes_ListProviders(t *testing.T) {
	usecase := &fakeAuthUseCase{oauthProviders: []domain.OAuthProviderInfo{{ID: "google", Name: "Google"}}}

	rec := httptest.NewRecorder()
	newOAuthTestRouter(usecase).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/providers", nil))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"google"`) {
		t.Fatalf("unexpected response %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOAuthRoutes_Authorize(t *testing.T) {
	usecase := &fakeAuthUseCase{oauthAuthorizeResp: &domain.OAuthAuthorization{URL: "https://x", State: "st", CodeVerifier: "cv"}}

	rec := httptest.NewRecorder()
	newOAuthTestRouter(usecase).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/providers/google/authorize", nil))

	if rec.Code != http.StatusOK || usecase.gotProviderID != "google" {
		t.Fatalf("unexpected response %d (provider %q): %s", rec.Code, usecase.gotProviderID, rec.Body.String())
	}
	var got domain.OAuthAuthorization
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.State != "st" || got.CodeVerifier != "cv" {
		t.Fatalf("unexpected body %s (%v)", rec.Body.String(), err)
	}
}

func TestOAuthRoutes_LoginPassesCallback(t *testing.T) {
	usecase := &fakeAuthUseCase{oauthLoginResp: &domain.LoginResponse{Token: "t", SessionID: "s1"}}
	body := `{"code":"c","state":"st","codeVerifier":"cv"}`

	rec := httptest.NewRecorder()
	newOAuthTestRouter(usecase).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/oauth/google/login", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	want := domain.OAuthCallback{Code: "c", State: "st", CodeVerifier: "cv"}
	if usecase.gotProviderID != "google" || usecase.gotCallback != want {
		t.Fatalf("unexpected usecase args: %q %+v", usecase.gotProviderID, usecase.gotCallback)
	}
}

func TestOAuthRoutes_RegisterPassesUsername(t *testing.T) {
	usecase := &fakeAuthUseCase{oauthRegisterResp: &domain.LoginResponse{Token: "t"}}
	body := `{"code":"c","state":"st","codeVerifier":"cv","username":"speedy"}`

	rec := httptest.NewRecorder()
	newOAuthTestRouter(usecase).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/oauth/google/register", strings.NewReader(body)))

	if rec.Code != http.StatusOK || usecase.gotUsername != "speedy" || usecase.gotCallback.Code != "c" {
		t.Fatalf("unexpected result %d, username %q, callback %+v", rec.Code, usecase.gotUsername, usecase.gotCallback)
	}
}

func TestOAuthRoutes_MalformedBody(t *testing.T) {
	rec := httptest.NewRecorder()
	newOAuthTestRouter(&fakeAuthUseCase{}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/oauth/google/login", strings.NewReader(`{nope`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestOAuthRoutes_ErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{domain.ErrUnknownOAuthProvider, http.StatusNotFound},
		{domain.ErrInvalidOAuthState, http.StatusUnauthorized},
		{fmt.Errorf("%w: google: boom", domain.ErrOAuthExchangeFailed), http.StatusUnauthorized},
		{domain.ErrOAuthEmailNotVerified, http.StatusForbidden},
		{domain.ErrOAuthAccountNotLinked, http.StatusNotFound},
		{domain.ErrOAuthAccountAlreadyLinked, http.StatusConflict},
		{domain.ErrEmailAlreadyRegistered, http.StatusConflict},
		{fmt.Errorf("db down"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/oauth/google/login", strings.NewReader(`{}`))
			newOAuthTestRouter(&fakeAuthUseCase{oauthErr: tc.err}).ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("expected %d, got %d: %s", tc.status, rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "boom") || strings.Contains(rec.Body.String(), "db down") {
				t.Fatalf("internal error detail leaked to client: %s", rec.Body.String())
			}
		})
	}
}
