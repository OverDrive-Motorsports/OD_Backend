/**
##
## OverDrive 2026
## All Technical rights reserved
##
## sse_flush_test.go - Verifies SSE frames flow through the full gateway
## middleware chain (auth, rate limit, logging, reverse proxy) incrementally
## rather than being buffered until the upstream response completes.
##
*/

package httpinbound

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"overdrive/gateway/internal/adapters/outbound/auth"
	"overdrive/gateway/internal/core/usecases"
)

// TestSSEStreamsIncrementallyThroughGateway proves that a real Server-Sent
// Events response from an upstream is delivered to the client frame-by-frame
// through the full middleware chain (RequestLogger -> RateLimit -> auth ->
// ReverseProxy), not buffered until the handler returns. It fails if the gap
// between receiving frame 1 and frame 2 is not clearly visible (i.e. if
// everything arrived at once at the end).
//
// This is a generic capability test, deliberately decoupled from any
// specific endpoint: no route in this codebase currently streams a response
// (race-data-service's SSE replay endpoint was replaced by a plain-JSON bulk
// dump), but the Flush() passthrough this test guards
// (logging_middleware.go's statusRecorder) must keep working for whichever
// future endpoint streams next — see that file for why it's easy to
// accidentally break.
func TestSSEStreamsIncrementallyThroughGateway(t *testing.T) {
	const interFrameDelay = 300 * time.Millisecond

	// Fake upstream emitting three SSE frames with a deliberate delay between
	// them, flushing after each.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		for i := 0; i < 3; i++ {
			if i > 0 {
				time.Sleep(interFrameDelay)
			}
			w.Write([]byte("event: telemetry\ndata: {\"tick\":" + string(rune('0'+i)) + "}\n\n"))
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	tokenValidator := auth.NewStaticTokenValidator("test-token")
	authorizeUseCase := usecases.NewAuthorizeRequestUseCase(tokenValidator)
	healthUseCase := usecases.NewCheckHealthUseCase()

	routes := map[string][]*url.URL{"/v1/race-data": {upstreamURL}}
	handler, err := NewHandler(routes, map[string][]*url.URL{}, 1000, 1000, healthUseCase, authorizeUseCase)
	if err != nil {
		t.Fatalf("build handler: %v", err)
	}

	gateway := httptest.NewServer(handler)
	defer gateway.Close()

	req, err := http.NewRequest(http.MethodGet, gateway.URL+"/v1/race-data/sse-passthrough-check", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-token")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	var frameTimes []time.Time
	for len(frameTimes) < 3 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read line: %v (frames seen so far: %d)", err, len(frameTimes))
		}
		if strings.HasPrefix(line, "event: telemetry") {
			frameTimes = append(frameTimes, time.Now())
		}
	}

	// If frames were buffered until the response ended, all three timestamps
	// would be captured back-to-back within microseconds of each other. We
	// assert the observed gaps are a meaningful fraction of the deliberate
	// upstream delay, proving each frame was flushed and delivered as soon as
	// the upstream produced it.
	gap1 := frameTimes[1].Sub(frameTimes[0])
	gap2 := frameTimes[2].Sub(frameTimes[1])
	minExpected := interFrameDelay / 2

	if gap1 < minExpected {
		t.Errorf("frame 1->2 arrived too fast (%s), expected >= %s: frames were buffered, not streamed", gap1, minExpected)
	}
	if gap2 < minExpected {
		t.Errorf("frame 2->3 arrived too fast (%s), expected >= %s: frames were buffered, not streamed", gap2, minExpected)
	}
	t.Logf("observed inter-frame gaps through gateway: %s, %s (upstream delay was %s)", gap1, gap2, interFrameDelay)
}
