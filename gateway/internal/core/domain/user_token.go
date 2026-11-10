/**
##
## OverDrive 2026
## All Technical rights reserved
##
## user_token.go - Reasons a user access token is rejected at the gateway.
##
*/

package domain

import "errors"

// Rejection reasons of a user access token, kept distinct so the client gets the right 401
// code: an expired token means "refresh it", the others mean "log in again".
var (
	ErrMissingUserToken = errors.New("missing bearer token")
	ErrUserTokenExpired = errors.New("token expired")
	ErrInvalidUserToken = errors.New("invalid token")
)
