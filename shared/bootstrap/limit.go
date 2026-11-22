/**
##
## OverDrive 2026
## All Technical rights reserved
##
## limit.go - Bounded reader for upstream HTTP responses.
##
*/

package bootstrap

import (
	"errors"
	"fmt"
	"io"
)

// ErrResponseTooLarge is returned by a reader built with LimitResponse once the upstream sent
// more than the allowed number of bytes.
var ErrResponseTooLarge = errors.New("upstream response exceeds the size limit")

// LimitResponse wraps an upstream response body so that reading more than maxBytes fails with
// ErrResponseTooLarge. Unlike io.LimitReader, which silently truncates (and lets a JSON decoder
// report a misleading "unexpected EOF"), the caller gets an explicit error and never holds more
// than maxBytes of the response.
func LimitResponse(body io.Reader, maxBytes int64) io.Reader {
	return &limitedResponse{body: body, remaining: maxBytes, max: maxBytes}
}

type limitedResponse struct {
	body      io.Reader
	remaining int64
	max       int64
}

// Read reads from the body until the limit, then reports ErrResponseTooLarge if the body still
// has data.
func (l *limitedResponse) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		var probe [1]byte
		n, err := l.body.Read(probe[:])
		if n > 0 {
			return 0, fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, l.max)
		}
		return 0, err
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.body.Read(p)
	l.remaining -= int64(n)
	return n, err
}
