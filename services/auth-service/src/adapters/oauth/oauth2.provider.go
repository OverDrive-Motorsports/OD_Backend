/**
##
## OverDrive 2026
## All Technical rights reserved
##
## oauth2.provider.go - Generic OAuth2 authorization-code (+PKCE) provider reused by every
## concrete provider (Google, F1TV, ...).
##
*/

package oauthadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"overdrive/services/auth-service/src/core/domain"
	"strings"
	"time"
)

// Config holds the OAuth2 client credentials and endpoints of one provider.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	// ExtraAuthParams are appended to the consent URL (e.g. Google's "prompt").
	ExtraAuthParams url.Values
}

// UserInfoMapper turns the raw JSON returned by the provider's user-info endpoint into a
// domain identity. It is the only provider-specific piece most providers need.
type UserInfoMapper func(body []byte) (*domain.OAuthIdentity, error)

// Provider is a ports.OAuthProvider for any standard OAuth2 provider exposing a user-info
// endpoint. Concrete providers are just constructors filling in Config and a UserInfoMapper.
type Provider struct {
	id         string
	name       string
	kind       domain.OAuthProviderType
	cfg        Config
	mapper     UserInfoMapper
	httpClient *http.Client
}

// NewProvider builds a generic OAuth2 provider.
func NewProvider(id string, name string, kind domain.OAuthProviderType, cfg Config, mapper UserInfoMapper) *Provider {
	return &Provider{
		id:         id,
		name:       name,
		kind:       kind,
		cfg:        cfg,
		mapper:     mapper,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// WithHTTPClient overrides the HTTP client (used by tests).
func (p *Provider) WithHTTPClient(client *http.Client) *Provider {
	p.httpClient = client
	return p
}

func (p *Provider) ID() string                     { return p.id }
func (p *Provider) Name() string                   { return p.name }
func (p *Provider) Type() domain.OAuthProviderType { return p.kind }

func (p *Provider) AuthCodeURL(state string, codeChallenge string) string {
	params := url.Values{}
	for key, values := range p.cfg.ExtraAuthParams {
		params[key] = values
	}
	params.Set("response_type", "code")
	params.Set("client_id", p.cfg.ClientID)
	params.Set("redirect_uri", p.cfg.RedirectURL)
	params.Set("scope", strings.Join(p.cfg.Scopes, " "))
	params.Set("state", state)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	separator := "?"
	if strings.Contains(p.cfg.AuthURL, "?") {
		separator = "&"
	}
	return p.cfg.AuthURL + separator + params.Encode()
}

func (p *Provider) ExchangeCode(ctx context.Context, code string, codeVerifier string) (*domain.OAuthIdentity, error) {
	accessToken, err := p.exchangeToken(ctx, code, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", domain.ErrOAuthExchangeFailed, p.id, err)
	}

	body, err := p.fetchUserInfo(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", domain.ErrOAuthExchangeFailed, p.id, err)
	}

	identity, err := p.mapper(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", domain.ErrOAuthExchangeFailed, p.id, err)
	}
	if identity.Subject == "" {
		return nil, fmt.Errorf("%w: %s: user info has no subject", domain.ErrOAuthExchangeFailed, p.id)
	}
	identity.Provider = p.kind
	return identity, nil
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (p *Provider) exchangeToken(ctx context.Context, code string, codeVerifier string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", p.cfg.RedirectURL)
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	body, status, err := p.do(req)
	if err != nil {
		return "", err
	}

	var token tokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("token endpoint returned invalid JSON (status %d)", status)
	}
	if status != http.StatusOK || token.Error != "" {
		return "", fmt.Errorf("token endpoint returned status %d: %s %s", status, token.Error, token.ErrorDescription)
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned no access_token")
	}
	return token.AccessToken, nil
}

func (p *Provider) fetchUserInfo(ctx context.Context, accessToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	body, status, err := p.do(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("user info endpoint returned status %d", status)
	}
	return body, nil
}

func (p *Provider) do(req *http.Request) ([]byte, int, error) {
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
