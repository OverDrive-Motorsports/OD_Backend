/**
##
## OverDrive 2026
## All Technical rights reserved
##
## registry.go - Builds the list of OAuth providers enabled through the environment.
##
*/

package oauthadapter

import (
	"log"
	"overdrive/services/auth-service/src/core/ports"
)

// providerFactory builds one provider from the environment. It returns nil when the provider
// is not configured, so a missing provider disables it instead of crashing the service.
type providerFactory func(getenv func(string) string) ports.OAuthProvider

// factories lists every supported provider. To add a provider: write <name>.provider.go with a
// constructor (usually NewProvider + a UserInfoMapper), add its ProviderType value to
// resources/schema.prisma and domain/oauth.entity.go, then append a factory here.
var factories = []providerFactory{
	func(getenv func(string) string) ports.OAuthProvider {
		clientID, secret, redirect := getenv("GOOGLE_OAUTH_CLIENT_ID"), getenv("GOOGLE_OAUTH_CLIENT_SECRET"), getenv("GOOGLE_OAUTH_REDIRECT_URL")
		if clientID == "" || secret == "" || redirect == "" {
			return nil
		}
		return NewGoogleProvider(clientID, secret, redirect)
	},
}

// LoadProviders returns every provider whose configuration is present in the environment.
func LoadProviders(getenv func(string) string) []ports.OAuthProvider {
	providers := make([]ports.OAuthProvider, 0, len(factories))
	for _, factory := range factories {
		if provider := factory(getenv); provider != nil {
			providers = append(providers, provider)
			log.Printf("oauth provider enabled: %s", provider.ID())
		}
	}
	return providers
}
