/**
##
## OverDrive 2026
## All Technical rights reserved
##
## token_validator.go - Declares token validation port used by authorization logic.
##
*/

// Package ports defines gateway core ports.

package ports

type TokenValidator interface {
	ValidateBearerHeader(header string) bool
}
