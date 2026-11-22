/**
##
## OverDrive 2026
## All Technical rights reserved
##
## write_deadline_middleware_test.go - Tests for the per-route write deadline extension.
##
*/

package httpinbound

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"overdrive/gateway/internal/core/domain"
	"overdrive/shared/bootstrap"
)

// serveWithWriteTimeout serves handler behind the gateway's RequestLogger (which wraps the
// ResponseWriter) on a server whose WriteTimeout is 150 ms.
func serveWithWriteTimeout(t *testing.T, handler http.Handler) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	limits := bootstrap.DefaultServerLimits()
	limits.WriteTimeout = 150 * time.Millisecond
	server := bootstrap.NewServer(listener.Addr().String(), RequestLogger(handler), limits)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- bootstrap.Run(ctx, server, listener, time.Second) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return "http://" + listener.Addr().String()
}

// fetch returns the body of a GET, or "" when the server cut the response.
func fetch(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(body)
}

// slowHandler answers "done" after 400 ms, longer than the test WriteTimeout.
var slowHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	time.Sleep(400 * time.Millisecond)
	_, _ = io.WriteString(w, "done")
})

// TestExtendWriteDeadline_MatchingSuffixOutlivesServerTimeout proves a path ending with the
// configured suffix gets its response through the logger wrapper, while another path of the
// same prefix is still cut by the server WriteTimeout.
func TestExtendWriteDeadline_MatchingSuffixOutlivesServerTimeout(t *testing.T) {
	handler := ExtendWriteDeadline(domain.SlowWrite{PathSuffix: "/race/replay", Timeout: 2 * time.Second}, slowHandler)
	base := serveWithWriteTimeout(t, handler)

	if got := fetch(t, base+"/v1/race-data/sessions/x/race/replay"); got != "done" {
		t.Fatalf("replay path = %q, want done", got)
	}
	if got := fetch(t, base+"/v1/race-data/sessions/x/race/laps"); got == "done" {
		t.Fatal("non-matching path outlived the server WriteTimeout")
	}
}

// TestExtendWriteDeadline_EmptySuffixCoversWholePrefix proves an empty suffix extends every
// path of the route (ingestion).
func TestExtendWriteDeadline_EmptySuffixCoversWholePrefix(t *testing.T) {
	handler := ExtendWriteDeadline(domain.SlowWrite{Timeout: 2 * time.Second}, slowHandler)
	base := serveWithWriteTimeout(t, handler)

	if got := fetch(t, base+"/v1/ingestion/providers/openf1/ingestions"); got != "done" {
		t.Fatalf("ingestion path = %q, want done", got)
	}
}
