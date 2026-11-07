/**
##
## OverDrive 2026
## All Technical rights reserved
##
## oauth2_provider_test.go - Tests for the generic OAuth2 provider and the Google mapping.
##
*/

package oauthadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// newTestGoogle points a Google provider at a fake token + userinfo server.
func newTestGoogle(t *testing.T, tokenStatus int, userInfo string) *Provider {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("bad form: %v", err)
		}
		want := map[string]string{
			"grant_type": "authorization_code", "code": "the-code", "code_verifier": "the-verifier",
			"client_id": "cid", "client_secret": "csecret", "redirect_uri": "https://app.test/cb",
		}
		for key, value := range want {
			if r.PostForm.Get(key) != value {
				t.Errorf("token request %s = %q, want %q", key, r.PostForm.Get(key), value)
			}
		}
		w.WriteHeader(tokenStatus)
		if tokenStatus != http.StatusOK {
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"at-123","token_type":"Bearer"}`))
	})
	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(userInfo))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	provider := NewGoogleProvider("cid", "csecret", "https://app.test/cb")
	provider.cfg.TokenURL = server.URL + "/token"
	provider.cfg.UserInfoURL = server.URL + "/userinfo"
	return provider.WithHTTPClient(server.Client())
}

func TestGoogleProvider_ExchangeCode_Success(t *testing.T) {
	provider := newTestGoogle(t, http.StatusOK, `{"sub":"123","email":"a@b.com","email_verified":true,"name":"Ann"}`)

	identity, err := provider.ExchangeCode(context.Background(), "the-code", "the-verifier")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := domain.OAuthIdentity{Provider: domain.OAuthProviderGoogle, Subject: "123", Email: "a@b.com", EmailVerified: true, DisplayName: "Ann"}
	if *identity != want {
		t.Fatalf("got %+v, want %+v", *identity, want)
	}
}

func TestGoogleProvider_ExchangeCode_TokenRejected(t *testing.T) {
	provider := newTestGoogle(t, http.StatusBadRequest, "")

	if _, err := provider.ExchangeCode(context.Background(), "the-code", "the-verifier"); !errors.Is(err, domain.ErrOAuthExchangeFailed) {
		t.Fatalf("expected ErrOAuthExchangeFailed, got %v", err)
	}
}

func TestGoogleProvider_ExchangeCode_MissingSubject(t *testing.T) {
	provider := newTestGoogle(t, http.StatusOK, `{"email":"a@b.com","email_verified":true}`)

	if _, err := provider.ExchangeCode(context.Background(), "the-code", "the-verifier"); !errors.Is(err, domain.ErrOAuthExchangeFailed) {
		t.Fatalf("expected ErrOAuthExchangeFailed, got %v", err)
	}
}

func TestGoogleProvider_AuthCodeURL(t *testing.T) {
	raw := NewGoogleProvider("cid", "csecret", "https://app.test/cb").AuthCodeURL("st", "ch")

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid URL: %v", err)
	}
	q := parsed.Query()
	checks := map[string]string{
		"client_id": "cid", "redirect_uri": "https://app.test/cb", "response_type": "code",
		"scope": "openid email profile", "state": "st", "code_challenge": "ch", "code_challenge_method": "S256",
	}
	for key, value := range checks {
		if q.Get(key) != value {
			t.Errorf("%s = %q, want %q", key, q.Get(key), value)
		}
	}
	if q.Has("client_secret") {
		t.Fatalf("client secret must never appear in the consent URL")
	}
}

func TestLoadProviders_OnlyEnablesConfiguredProviders(t *testing.T) {
	if got := LoadProviders(func(string) string { return "" }); len(got) != 0 {
		t.Fatalf("expected no providers, got %d", len(got))
	}

	env := map[string]string{
		"GOOGLE_OAUTH_CLIENT_ID": "cid", "GOOGLE_OAUTH_CLIENT_SECRET": "s", "GOOGLE_OAUTH_REDIRECT_URL": "https://app.test/cb",
	}
	got := LoadProviders(func(key string) string { return env[key] })
	if len(got) != 1 || got[0].ID() != "google" {
		t.Fatalf("expected google to be enabled, got %+v", got)
	}
}
