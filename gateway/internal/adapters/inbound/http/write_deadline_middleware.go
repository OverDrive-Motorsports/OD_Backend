/**
##
## OverDrive 2026
## All Technical rights reserved
##
## write_deadline_middleware.go - Per-route write deadline extension for slow proxied responses.
##
*/

package httpinbound

import (
	"net/http"
	"strings"

	"overdrive/gateway/internal/core/domain"
	"overdrive/shared/bootstrap"
)

// ExtendWriteDeadline gives the requests matching slow (path suffix, or every path when the
// suffix is empty) a write deadline of slow.Timeout instead of the server WriteTimeout. Other
// requests keep the short server default.
func ExtendWriteDeadline(slow domain.SlowWrite, next http.Handler) http.Handler {
	extended := bootstrap.ExtendDeadlines(next, 0, slow.Timeout)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slow.PathSuffix == "" || strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), slow.PathSuffix) {
			extended.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
