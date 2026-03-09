/**
##
## OverDrive 2026
## All Technical rights reserved
##
## http_transport.go - HTTP transport helpers used to mock outbound provider calls in unit tests.
##
*/

package mocks

import (
	"io"
	"net/http"
	"strings"
)

// RoundTripFunc adapts a function into an http.RoundTripper for unit tests.
type RoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip executes the wrapped function for one outbound HTTP request.
func (fn RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// JSONResponse builds an HTTP response with a JSON body for transport mocks.
func JSONResponse(status int, body string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
