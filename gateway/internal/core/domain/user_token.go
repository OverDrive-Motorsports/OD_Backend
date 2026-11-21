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

// Rejection reasons of a user access token, kept distinct so the client gets the right code:
// an expired token means "refresh it", the other 401s mean "log in again", and
// ErrForbiddenRole (403) means the token is valid but its role does not open the route.
var (
	ErrMissingUserToken = errors.New("missing bearer token")
	ErrUserTokenExpired = errors.New("token expired")
	ErrInvalidUserToken = errors.New("invalid token")
	ErrForbiddenRole    = errors.New("role not allowed")
)

// UserRole is the role carried by an access token.
type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)
