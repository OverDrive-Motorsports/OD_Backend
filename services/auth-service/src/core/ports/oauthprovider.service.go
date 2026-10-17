/**
##
## OverDrive 2026
## All Technical rights reserved
##
## oauthprovider.service.go - Package ports source file for services/auth-service/src/core/ports.
##
*/

package ports

type OAuthProvider interface {
	GetName() string
	GetAuthURL() string
	ExchangeCodeForToken(code string) (string, error)
}
