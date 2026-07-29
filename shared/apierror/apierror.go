/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## apierror.go - Package apierror source file for shared/apierror.
	##
*/

package apierror

import "strings"

// Code is the machine-readable error identifier shared across services (SCREAMING_SNAKE_CASE).
type Code string

// Status is the HTTP status code associated with a Code.
type Status int

// Predefined error codes. One per cause, not per endpoint - reuse across services instead of
// inventing new codes for the same situation.
const (
	CodeValidationError     Code = "VALIDATION_ERROR"
	CodeUnauthorized        Code = "UNAUTHORIZED"
	CodeInvalidToken        Code = "INVALID_TOKEN"
	CodeTokenExpired        Code = "TOKEN_EXPIRED"
	CodeForbidden           Code = "FORBIDDEN"
	CodeMethodNotAllowed    Code = "METHOD_NOT_ALLOWED"
	CodeRateLimitExceeded   Code = "RATE_LIMIT_EXCEEDED"
	CodeInternalError       Code = "INTERNAL_ERROR"
	CodeUpstreamUnavailable Code = "UPSTREAM_UNAVAILABLE"
	CodeServiceUnavailable  Code = "SERVICE_UNAVAILABLE"
	CodeUpstreamTimeout     Code = "UPSTREAM_TIMEOUT"
)

// HTTP statuses used by this package. Kept as named constants so call sites never hardcode numbers.
const (
	StatusBadRequest          Status = 400
	StatusUnauthorized        Status = 401
	StatusForbidden           Status = 403
	StatusNotFound            Status = 404
	StatusMethodNotAllowed    Status = 405
	StatusConflict            Status = 409
	StatusTooManyRequests     Status = 429
	StatusInternalServerError Status = 500
	StatusBadGateway          Status = 502
	StatusServiceUnavailable  Status = 503
	StatusGatewayTimeout      Status = 504
)

// Error is the standardized error carried from a handler down to the HTTP response and the
// terminal log. Message is client-safe; Err carries the technical detail that must never reach
// the client and is only ever surfaced through LogDebug.
type Error struct {
	Code    Code
	Status  Status
	Message string
	Err     error
}

// New builds an Error from an explicit code/status pair.
func New(code Code, status Status, message string, err error) *Error {
	return &Error{Code: code, Status: status, Message: message, Err: err}
}

// Error implements the error interface, returning the client-facing message.
func (e *Error) Error() string {
	return e.Message
}

// Unwrap exposes the wrapped technical error to errors.Is/errors.As.
func (e *Error) Unwrap() error {
	return e.Err
}

// Validation reports an invalid parameter, body, or query (400).
func Validation(message string, err error) *Error {
	return New(CodeValidationError, StatusBadRequest, message, err)
}

// InvalidField reports an invalid named field (400), e.g. InvalidField("EMAIL", ...) -> INVALID_EMAIL.
func InvalidField(field string, message string, err error) *Error {
	return New(resourceCode(field, "INVALID", true), StatusBadRequest, message, err)
}

// Unauthorized reports a missing or invalid authentication (401).
func Unauthorized(message string, err error) *Error {
	return New(CodeUnauthorized, StatusUnauthorized, message, err)
}

// InvalidToken reports an invalid auth token (401).
func InvalidToken(message string, err error) *Error {
	return New(CodeInvalidToken, StatusUnauthorized, message, err)
}

// TokenExpired reports an expired auth token (401).
func TokenExpired(message string, err error) *Error {
	return New(CodeTokenExpired, StatusUnauthorized, message, err)
}

// Forbidden reports a denied access to an otherwise valid request (403).
func Forbidden(message string, err error) *Error {
	return New(CodeForbidden, StatusForbidden, message, err)
}

// NotFound reports a missing resource (404), e.g. NotFound("RACE", ...) -> RACE_NOT_FOUND.
func NotFound(resource string, message string, err error) *Error {
	return New(resourceCode(resource, "NOT_FOUND", false), StatusNotFound, message, err)
}

// MethodNotAllowed reports an unsupported HTTP verb for the route (405).
func MethodNotAllowed(message string, err error) *Error {
	return New(CodeMethodNotAllowed, StatusMethodNotAllowed, message, err)
}

// Conflict reports a conflicting state for a resource (409), e.g. Conflict("USER", ...) -> USER_CONFLICT.
func Conflict(resource string, message string, err error) *Error {
	return New(resourceCode(resource, "CONFLICT", false), StatusConflict, message, err)
}

// AlreadyExists reports a duplicate resource (409), e.g. AlreadyExists("SESSION", ...) -> SESSION_ALREADY_EXISTS.
func AlreadyExists(resource string, message string, err error) *Error {
	return New(resourceCode(resource, "ALREADY_EXISTS", false), StatusConflict, message, err)
}

// RateLimitExceeded reports a client exceeding its allowed request rate (429).
func RateLimitExceeded(message string, err error) *Error {
	return New(CodeRateLimitExceeded, StatusTooManyRequests, message, err)
}

// Internal reports an unanticipated bug (500).
func Internal(message string, err error) *Error {
	return New(CodeInternalError, StatusInternalServerError, message, err)
}

// UpstreamUnavailable reports a dependent service that could not be reached (502).
func UpstreamUnavailable(message string, err error) *Error {
	return New(CodeUpstreamUnavailable, StatusBadGateway, message, err)
}

// ServiceUnavailable reports a failed healthcheck for the current service (503).
func ServiceUnavailable(message string, err error) *Error {
	return New(CodeServiceUnavailable, StatusServiceUnavailable, message, err)
}

// UpstreamTimeout reports a timed out call to an upstream service (504).
func UpstreamTimeout(message string, err error) *Error {
	return New(CodeUpstreamTimeout, StatusGatewayTimeout, message, err)
}

// resourceCode builds a SCREAMING_SNAKE_CASE code from a resource/field name and a suffix,
// e.g. resourceCode("race", "NOT_FOUND", false) -> "RACE_NOT_FOUND"
// and resourceCode("email", "INVALID", true) -> "INVALID_EMAIL".
func resourceCode(name string, suffix string, prefixed bool) Code {
	name = strings.ToUpper(strings.TrimSpace(name))
	if prefixed {
		return Code(suffix + "_" + name)
	}
	return Code(name + "_" + suffix)
}
