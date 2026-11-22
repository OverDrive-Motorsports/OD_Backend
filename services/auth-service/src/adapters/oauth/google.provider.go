/**
##
## OverDrive 2026
## All Technical rights reserved
##
## google.provider.go - Google OAuth provider (OpenID Connect).
##
*/

package oauthadapter

import (
	"encoding/json"
	"net/url"
	"overdrive/services/auth-service/src/core/domain"
)

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

// NewGoogleProvider builds the Google provider. Only credentials and the redirect URL are
// configurable; endpoints and scopes are Google's.
func NewGoogleProvider(clientID string, clientSecret string, redirectURL string) *Provider {
	return NewProvider("google", "Google", domain.OAuthProviderGoogle, Config{
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURL:     redirectURL,
		AuthURL:         googleAuthURL,
		TokenURL:        googleTokenURL,
		UserInfoURL:     googleUserInfoURL,
		Scopes:          []string{"openid", "email", "profile"},
		ExtraAuthParams: url.Values{"prompt": {"select_account"}},
	}, mapGoogleUserInfo)
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func mapGoogleUserInfo(body []byte) (*domain.OAuthIdentity, error) {
	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &domain.OAuthIdentity{
		Subject:       info.Sub,
		Email:         info.Email,
		EmailVerified: info.EmailVerified,
		DisplayName:   info.Name,
	}, nil
}
