/**
##
## OverDrive 2026
## All Technical rights reserved
##
## errors.go - Package domain source file for services/auth-service/src/core/domain.
##
*/

package domain

import "errors"

// ErrInvalidCredentials is returned by AuthQueryUseCase.Login for BOTH "user not found" and
// "wrong password" cases. This is intentional anti-enumeration: the client-facing message must
// stay identical for both, so adapters/http must classify it with errors.Is instead of a raw
// string message, and must never split it into two different codes/messages.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrEmailAlreadyRegistered is returned by AuthQueryUseCase.Register when the requested email
// already has an account.
var ErrEmailAlreadyRegistered = errors.New("email already registered")

// ErrInvalidSession is returned by AuthQueryUseCase.Refresh when the supplied sessionID does not
// resolve to a known session (or its owning user no longer exists).
var ErrInvalidSession = errors.New("invalid session")

// ErrInvalidToken is returned by AuthQueryUseCase.Refresh/VerifyToken when the supplied
// refresh/bearer token does not match or fails validation.
var ErrInvalidToken = errors.New("invalid token")

// ErrUnknownOAuthProvider is returned when the {providerId} path segment does not match any
// configured OAuthProvider.
var ErrUnknownOAuthProvider = errors.New("unknown oauth provider")

// ErrInvalidOAuthState is returned when the state sent back by the client is missing, forged,
// expired, or was issued for another provider.
var ErrInvalidOAuthState = errors.New("invalid oauth state")

// ErrOAuthExchangeFailed is returned when the provider rejects the authorization code or its
// user profile cannot be fetched.
var ErrOAuthExchangeFailed = errors.New("oauth code exchange failed")

// ErrOAuthEmailNotVerified is returned when the provider does not vouch for the user's email.
// Accounts are keyed by email, so an unverified one could be used to squat another user's address.
var ErrOAuthEmailNotVerified = errors.New("oauth email not verified")

// ErrOAuthAccountNotLinked is returned by OAuthLogin when no user is linked to this provider
// identity yet; the client should offer OAuthRegister.
var ErrOAuthAccountNotLinked = errors.New("oauth account not linked")

// ErrOAuthAccountAlreadyLinked is returned by OAuthRegister when this provider identity already
// belongs to a user; the client should use OAuthLogin.
var ErrOAuthAccountAlreadyLinked = errors.New("oauth account already linked")
