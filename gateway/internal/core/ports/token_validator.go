package ports

type TokenValidator interface {
	ValidateBearerHeader(header string) bool
}
