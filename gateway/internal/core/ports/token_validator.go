// Package ports defines gateway core ports.

package ports

type TokenValidator interface {
	ValidateBearerHeader(header string) bool
}
