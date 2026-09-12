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
