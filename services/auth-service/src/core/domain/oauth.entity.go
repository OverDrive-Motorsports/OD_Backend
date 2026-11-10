/**
##
## OverDrive 2026
## All Technical rights reserved
##
## oauth.entity.go - Domain entities shared by every OAuth provider of the auth-service.
##
*/

package domain

// OAuthProviderType is the persisted identifier of an OAuth provider. Values MUST match the
// ProviderType enum of resources/schema.prisma (the generated db.ProviderType constants).
type OAuthProviderType string

const (
	OAuthProviderGoogle OAuthProviderType = "GOOGLE"
	OAuthProviderF1TV   OAuthProviderType = "F1TV"
)

// OAuthIdentity is the provider-agnostic user profile every OAuthProvider returns once an
// authorization code has been exchanged. Subject is the provider's stable user ID (OIDC "sub");
// it, not the email, is what links an OAuthAccount to a User.
type OAuthIdentity struct {
	Provider      OAuthProviderType
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

// OAuthProviderInfo describes a configured provider to clients (GET /oauth/providers).
type OAuthProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OAuthAuthorization is returned when a client starts an OAuth flow. The client redirects the
// user to URL, keeps State and CodeVerifier, and sends them back with the authorization code.
type OAuthAuthorization struct {
	URL          string `json:"url"`
	State        string `json:"state"`
	CodeVerifier string `json:"codeVerifier"`
}

// OAuthCallback is what the client received from the provider redirect, plus the PKCE
// verifier it kept from OAuthAuthorization.
type OAuthCallback struct {
	Code         string
	State        string
	CodeVerifier string
}
