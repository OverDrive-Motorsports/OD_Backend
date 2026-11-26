/**
##
## OverDrive 2026
## All Technical rights reserved
##
## token.go - OpenF1 access token: obtained with the account credentials and renewed before it expires.
##
*/

package openf1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
	"overdrive/shared/bootstrap"
)

const (
	maxTokenResponseBytes int64 = 64 << 10
	// tokenMargin renews the token before its real expiry.
	tokenMargin = time.Minute
)

// tokenSource exchanges the OpenF1 account credentials for an access token and caches it. Neither
// the credentials nor the token are ever logged.
type tokenSource struct {
	tokenURL   string
	username   string
	password   string
	httpClient *http.Client
	now        func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// newTokenSource builds the source, or returns nil without credentials (anonymous access).
func newTokenSource(tokenURL string, username string, password string, timeout time.Duration) *tokenSource {
	if username == "" || password == "" {
		return nil
	}
	return &tokenSource{
		tokenURL: tokenURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: timeout,
			// The credentials are only ever sent to the configured URL.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now: time.Now,
	}
}

// get returns a valid token, requesting one when needed.
func (s *tokenSource) get(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && s.now().Before(s.expires) {
		return s.token, nil
	}
	form := url.Values{"username": {s.username}, "password": {s.password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("invalid OpenF1 token URL")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenF1 token request: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnprocessableEntity:
		return "", domain.ErrOpenF1Credentials
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("OpenF1 token request failed with status %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		// ExpiresIn is documented as a string; a number is accepted too.
		ExpiresIn any `json:"expires_in"`
	}
	if err := json.NewDecoder(bootstrap.LimitResponse(resp.Body, maxTokenResponseBytes)).Decode(&body); err != nil {
		return "", errors.New("invalid OpenF1 token response")
	}
	if body.AccessToken == "" {
		return "", errors.New("OpenF1 token response without token")
	}
	lifetime := time.Duration(intValue(body.ExpiresIn)) * time.Second
	if lifetime <= tokenMargin {
		return "", fmt.Errorf("OpenF1 token lifetime too short: %s", lifetime)
	}
	s.token, s.expires = body.AccessToken, s.now().Add(lifetime-tokenMargin)
	return s.token, nil
}

// discard forgets the token OpenF1 refused, unless another call already renewed it.
func (s *tokenSource) discard(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == token {
		s.token = ""
	}
}
