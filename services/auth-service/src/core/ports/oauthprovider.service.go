/**
##
## OverDrive 2026
## All Technical rights reserved
##
## oauthprovider.service.go - Port every OAuth provider adapter (Google, F1TV, ...) implements.
##
*/

package ports

import (
	"context"
	"overdrive/services/auth-service/src/core/domain"
)

// OAuthProvider is the only thing the auth use case knows about a provider. Adding a provider
// means writing one adapter implementing this interface and registering it in
// src/adapters/oauth/registry.go; nothing in core/ or adapters/http changes.
type OAuthProvider interface {
	// ID is the URL-safe identifier used in routes, e.g. "google" in /oauth/google/login.
	ID() string
	// Name is the human-readable label shown by clients, e.g. "Google".
	Name() string
	// Type is the value persisted in OAuthAccount.provider.
	Type() domain.OAuthProviderType
	// AuthCodeURL builds the provider consent URL for the given state and PKCE S256 challenge.
	AuthCodeURL(state string, codeChallenge string) string
	// ExchangeCode trades an authorization code (and its PKCE verifier) for the user identity.
	ExchangeCode(ctx context.Context, code string, codeVerifier string) (*domain.OAuthIdentity, error)
}
