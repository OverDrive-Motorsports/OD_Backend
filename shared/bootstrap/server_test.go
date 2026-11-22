/**
##
## OverDrive 2026
## All Technical rights reserved
##
## server_test.go - Tests for the shared HTTP server limits, deadlines and graceful shutdown.
##
*/

package bootstrap

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

// testLimits are short versions of the default limits so timeouts trigger within a test.
func testLimits() ServerLimits {
	return ServerLimits{
		ReadHeaderTimeout: 100 * time.Millisecond,
		ReadTimeout:       200 * time.Millisecond,
		WriteTimeout:      200 * time.Millisecond,
		IdleTimeout:       200 * time.Millisecond,
		MaxHeaderBytes:    4 << 10,
	}
}

// startServer serves handler with limits on a random local port until the test ends.
func startServer(t *testing.T, handler http.Handler, limits ServerLimits) (*http.Server, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := NewServer(listener.Addr().String(), handler, limits)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, listener, time.Second) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return server, listener.Addr().String()
}

// waitClosed reports whether the server closes conn within limit (the read returns EOF or a
// reset instead of timing out on the client side).
func waitClosed(t *testing.T, conn net.Conn, limit time.Duration) bool {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(limit))
	_, err := io.ReadAll(conn)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	return true
}

// TestDefaultServerLimits_AllSet proves every server gets read, write and idle timeouts and a
// header size cap, with WriteTimeout above the 30 s race control long poll.
func TestDefaultServerLimits_AllSet(t *testing.T) {
	limits := DefaultServerLimits()
	server := NewServer(":0", http.NotFoundHandler(), limits)

	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("missing timeout: %+v", limits)
	}
	if server.MaxHeaderBytes <= 0 {
		t.Fatalf("MaxHeaderBytes not set")
	}
	if server.WriteTimeout <= 30*time.Second {
		t.Fatalf("WriteTimeout %s must exceed the 30s long poll", server.WriteTimeout)
	}
}

// TestNewServer_SlowHeadersAreCut proves a client that never finishes its request headers is
// disconnected after ReadHeaderTimeout instead of holding the connection.
func TestNewServer_SlowHeadersAreCut(t *testing.T) {
	_, addr := startServer(t, http.NotFoundHandler(), testLimits())
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !waitClosed(t, conn, 2*time.Second) {
		t.Fatal("connection still open after ReadHeaderTimeout")
	}
}

// TestNewServer_SlowBodyIsCut proves a client that sends its body too slowly makes the body read
// fail after ReadTimeout, so the handler is released.
func TestNewServer_SlowBodyIsCut(t *testing.T) {
	readErr := make(chan error, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		readErr <- err
	})
	_, addr := startServer(t, handler, testLimits())
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\n0123456789")); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("body read succeeded with 90 bytes missing")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler still blocked on the body after ReadTimeout")
	}
}

// TestNewServer_IdleConnectionIsClosed proves a keep-alive connection left idle is closed after
// IdleTimeout.
func TestNewServer_IdleConnectionIsClosed(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	_, addr := startServer(t, handler, testLimits())
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	_ = resp.Body.Close()
	if !waitClosed(t, conn, 2*time.Second) {
		t.Fatal("idle keep-alive connection still open after IdleTimeout")
	}
}

// TestNewServer_OversizedHeadersRejected proves headers above MaxHeaderBytes get 431 instead of
// being buffered.
func TestNewServer_OversizedHeadersRejected(t *testing.T) {
	_, addr := startServer(t, http.NotFoundHandler(), testLimits())
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/", nil)
	req.Header.Set("X-Big", strings.Repeat("a", 16<<10))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status = %d, want 431", resp.StatusCode)
	}
}

// TestExtendDeadlines_OutlivesWriteTimeout proves a slow route answers when wrapped with
// ExtendDeadlines, while the same handler without it is cut by the server WriteTimeout.
func TestExtendDeadlines_OutlivesWriteTimeout(t *testing.T) {
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	})
	mux := http.NewServeMux()
	mux.Handle("/plain", slow)
	mux.Handle("/extended", ExtendDeadlines(slow, 0, 2*time.Second))
	_, addr := startServer(t, mux, testLimits())

	if resp, err := http.Get("http://" + addr + "/plain"); err == nil {
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr == nil && string(body) == "done" {
			t.Fatal("/plain answered after the WriteTimeout")
		}
	}

	resp, err := http.Get("http://" + addr + "/extended")
	if err != nil {
		t.Fatalf("/extended: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "done" {
		t.Fatalf("/extended = %d %q, want 200 done", resp.StatusCode, body)
	}
}

// TestRun_InFlightRequestFinishes proves that when the context ends (SIGTERM), a request already
// running completes with its response and Run returns nil.
func TestRun_InFlightRequestFinishes(t *testing.T) {
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "finished")
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := NewServer(listener.Addr().String(), handler, DefaultServerLimits())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, listener, 2*time.Second) }()

	type result struct {
		body string
		err  error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String() + "/")
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		got <- result{body: string(body), err: err}
	}()

	<-started
	cancel()

	res := <-got
	if res.err != nil || res.body != "finished" {
		t.Fatalf("in-flight request = %q, %v; want finished", res.body, res.err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
}

// TestRun_DeadlineExceededReturnsError proves a request still running at the shutdown deadline
// is cut and Run reports it instead of hanging.
func TestRun_DeadlineExceededReturnsError(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})
	defer close(release)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := NewServer(listener.Addr().String(), handler, DefaultServerLimits())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, listener, 100*time.Millisecond) }()
	go func() {
		if resp, err := http.Get("http://" + listener.Addr().String() + "/"); err == nil {
			_ = resp.Body.Close()
		}
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run() = nil, want a deadline error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after the shutdown deadline")
	}
}

// TestServe_SIGTERMLetsInFlightRequestFinish proves the real signal path: SIGTERM sent to the
// process while a request runs lets that request finish, then Serve returns nil.
func TestServe_SIGTERMLetsInFlightRequestFinish(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()

	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			close(started)
			time.Sleep(300 * time.Millisecond)
		}
		_, _ = io.WriteString(w, "ok")
	})
	server := NewServer(addr, handler, DefaultServerLimits())
	done := make(chan error, 1)
	go func() { done <- Serve(server, 2*time.Second) }()

	// Serve registers the signal handler before listening: once a request succeeds, SIGTERM is
	// caught by Serve instead of killing the test binary.
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + "/ready")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never became ready: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	got := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			got <- "error: " + err.Error()
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		got <- string(body)
	}()
	<-started
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("kill: %v", err)
	}

	if body := <-got; body != "ok" {
		t.Fatalf("in-flight request = %q, want ok", body)
	}
	if err := <-done; err != nil {
		t.Fatalf("Serve() = %v, want nil", err)
	}
}

// TestShutdownTimeout covers the default, a valid override and the rejected values of
// SHUTDOWN_TIMEOUT.
func TestShutdownTimeout(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{"unset uses default", "", DefaultShutdownTimeout, false},
		{"valid override", "45s", 45 * time.Second, false},
		{"not a duration", "soon", 0, true},
		{"zero", "0s", 0, true},
		{"negative", "-5s", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SHUTDOWN_TIMEOUT", tc.value)
			got, err := ShutdownTimeout()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
