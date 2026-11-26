/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live_test.go - Unit tests for the OpenF1 live calls and the access token.
##
*/

package openf1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
)

// liveClient builds a client on the test server, without pacing nor retry delay.
func liveClient(server *httptest.Server, retries int, options ClientOptions) *Client {
	return NewClient(server.URL+"/v1", 2*time.Second, retries, time.Millisecond, options)
}

// TestFindSession proves the session is read with its dates in UTC, and that an unknown session, an
// empty answer or a session without usable dates is domain.ErrLiveSessionNotFound.
func TestFindSession(t *testing.T) {
	body, status := "", http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions" || r.URL.Query().Get("session_key") != "9998" {
			t.Errorf("unexpected request %s", r.URL)
		}
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	client := liveClient(server, 0, ClientOptions{})

	body = `[{"session_key":9998,"meeting_key":1255,"date_start":"2025-03-23T15:00:00+08:00","date_end":"2025-03-23T17:00:00+08:00"}]`
	session, err := client.FindSession(context.Background(), 9998)
	want := domain.LiveSession{SessionKey: 9998, MeetingKey: 1255,
		StartUTC: time.Date(2025, 3, 23, 7, 0, 0, 0, time.UTC), EndUTC: time.Date(2025, 3, 23, 9, 0, 0, 0, time.UTC)}
	if err != nil || session != want {
		t.Fatalf("session=%+v err=%v", session, err)
	}
	for _, bad := range []string{`[]`, `[{"meeting_key":1255,"date_start":"soon","date_end":"2025-03-23T17:00:00+08:00"}]`,
		`[{"date_start":"2025-03-23T15:00:00Z","date_end":"2025-03-23T17:00:00Z"}]`} {
		body = bad
		if _, err := client.FindSession(context.Background(), 9998); !errors.Is(err, domain.ErrLiveSessionNotFound) {
			t.Fatalf("%s: err=%v", bad, err)
		}
	}
	status, body = http.StatusNotFound, `{"detail":"No results found."}`
	if _, err := client.FindSession(context.Background(), 9998); !errors.Is(err, domain.ErrLiveSessionNotFound) {
		t.Fatalf("404: err=%v", err)
	}
	status = http.StatusBadRequest
	if _, err := client.FindSession(context.Background(), 9998); err == nil || errors.Is(err, domain.ErrLiveSessionNotFound) {
		t.Fatalf("400: err=%v", err)
	}
}

// TestFetchWindow proves the bounds become OpenF1 comparison filters on the resource's date field
// in UTC, that an open window or a resource without date sends none, and that a 404 is no rows.
func TestFetchWindow(t *testing.T) {
	var query string
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.WriteHeader(status)
		fmt.Fprint(w, `[{"driver_number":1}]`)
	}))
	defer server.Close()
	client := liveClient(server, 0, ClientOptions{})
	from := time.Date(2025, 3, 23, 8, 0, 0, 123000000, time.FixedZone("", 3600))
	to := from.Add(time.Minute)

	for _, tc := range []struct {
		resource domain.LiveResource
		window   domain.LiveWindow
		want     string
	}{
		{domain.LiveResource{Name: "car_data", DateField: "date"}, domain.LiveWindow{From: &from, To: &to},
			"date%3C=2025-03-23T07%3A01%3A00.123&date%3E=2025-03-23T07%3A00%3A00.123&session_key=9998"},
		{domain.LiveResource{Name: "laps", DateField: "date_start"}, domain.LiveWindow{From: &from}, "date_start%3E=2025-03-23T07%3A00%3A00.123&session_key=9998"},
		{domain.LiveResource{Name: "laps", DateField: "date_start"}, domain.LiveWindow{}, "session_key=9998"},
		{domain.LiveResource{Name: "stints"}, domain.LiveWindow{From: &from, To: &to}, "session_key=9998"},
	} {
		rows, err := client.FetchWindow(context.Background(), tc.resource, 9998, tc.window)
		if err != nil || len(rows) != 1 || query != tc.want {
			t.Fatalf("%s: rows=%v err=%v query=%s, want %s", tc.resource.Name, rows, err, query, tc.want)
		}
	}
	status = http.StatusNotFound
	if rows, err := client.FetchWindow(context.Background(), domain.LiveResource{Name: "laps"}, 1, domain.LiveWindow{}); err != nil || len(rows) != 0 {
		t.Fatalf("404: rows=%v err=%v", rows, err)
	}
}

// tokenServer is a fake OpenF1 with a token endpoint. Data calls need an accepted token.
type tokenServer struct {
	tokenStatus int
	tokenBody   string
	tokenCalls  int
	dataCalls   int
	accept      func(authorization string) bool
	form        string
}

func (s *tokenServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		s.tokenCalls++
		_ = r.ParseForm()
		s.form = r.Method + " " + r.PostForm.Encode()
		w.WriteHeader(s.tokenStatus)
		fmt.Fprint(w, strings.ReplaceAll(s.tokenBody, "N", fmt.Sprint(s.tokenCalls)))
		return
	}
	s.dataCalls++
	if !s.accept(r.Header.Get("Authorization")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	fmt.Fprint(w, `[{"driver_number":1}]`)
}

// TestToken_SentAndCached proves the credentials are exchanged once and the token sent on every
// call, until shortly before its expiry.
func TestToken_SentAndCached(t *testing.T) {
	fake := &tokenServer{tokenStatus: http.StatusOK, tokenBody: `{"access_token":"token-N","expires_in":"3600","token_type":"bearer"}`,
		accept: func(authorization string) bool { return strings.HasPrefix(authorization, "Bearer token-") }}
	server := httptest.NewServer(fake)
	defer server.Close()
	client := liveClient(server, 0, ClientOptions{Username: "me@example.com", Password: "p&w", TokenURL: server.URL + "/token"})
	now := time.Now()
	client.tokens.now = func() time.Time { return now }

	fetch := func() {
		t.Helper()
		if _, err := client.FetchWindow(context.Background(), domain.LiveResource{Name: "laps"}, 1, domain.LiveWindow{}); err != nil {
			t.Fatal(err)
		}
	}
	fetch()
	fetch()
	if fake.tokenCalls != 1 || fake.form != "POST password=p%26w&username=me%40example.com" {
		t.Fatalf("token calls=%d form=%q", fake.tokenCalls, fake.form)
	}
	now = now.Add(59*time.Minute + time.Second)
	fetch()
	if fake.tokenCalls != 2 {
		t.Fatalf("token calls=%d, want a renewal before expiry", fake.tokenCalls)
	}
}

// TestToken_RenewedOnce proves a refused token is renewed once without consuming a retry, and that
// a second refusal is domain.ErrOpenF1Credentials instead of a loop.
func TestToken_RenewedOnce(t *testing.T) {
	fake := &tokenServer{tokenStatus: http.StatusOK, tokenBody: `{"access_token":"token-N","expires_in":3600}`,
		accept: func(authorization string) bool { return authorization == "Bearer token-2" }}
	server := httptest.NewServer(fake)
	defer server.Close()
	client := liveClient(server, 0, ClientOptions{Username: "u", Password: "p", TokenURL: server.URL + "/token"})
	if _, err := client.FetchWindow(context.Background(), domain.LiveResource{Name: "laps"}, 1, domain.LiveWindow{}); err != nil || fake.tokenCalls != 2 {
		t.Fatalf("err=%v token calls=%d", err, fake.tokenCalls)
	}
	fake.accept = func(string) bool { return false }
	before := fake.dataCalls
	_, err := client.FetchWindow(context.Background(), domain.LiveResource{Name: "laps"}, 1, domain.LiveWindow{})
	if !errors.Is(err, domain.ErrOpenF1Credentials) || fake.dataCalls-before != 2 {
		t.Fatalf("err=%v data calls=%d", err, fake.dataCalls-before)
	}
}

// TestToken_Failures proves refused credentials fail at once without any data call, other token
// failures are ordinary errors, and no error ever contains the password or the response body.
func TestToken_Failures(t *testing.T) {
	for _, tc := range []struct {
		name, body  string
		status      int
		credentials bool
	}{
		{"wrong credentials", `{"detail":"s3cret"}`, http.StatusUnauthorized, true},
		{"bad request", `s3cret`, http.StatusBadRequest, true},
		{"server error", `s3cret`, http.StatusInternalServerError, false},
		{"redirect", ``, http.StatusTemporaryRedirect, false},
		{"not json", `s3cret`, http.StatusOK, false},
		{"no token", `{"expires_in":"3600"}`, http.StatusOK, false},
		{"no lifetime", `{"access_token":"s3cret"}`, http.StatusOK, false},
	} {
		fake := &tokenServer{tokenStatus: tc.status, tokenBody: tc.body, accept: func(string) bool { return true }}
		server := httptest.NewServer(fake)
		client := liveClient(server, 0, ClientOptions{Username: "u", Password: "s3cret", TokenURL: server.URL + "/token"})
		_, err := client.FetchWindow(context.Background(), domain.LiveResource{Name: "laps"}, 1, domain.LiveWindow{})
		server.Close()
		if err == nil || errors.Is(err, domain.ErrOpenF1Credentials) != tc.credentials || strings.Contains(err.Error(), "s3cret") {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
		if fake.dataCalls != 0 || fake.tokenCalls != 1 {
			t.Fatalf("%s: %d data calls, %d token calls", tc.name, fake.dataCalls, fake.tokenCalls)
		}
	}
}

// TestToken_Anonymous proves nothing is sent without an account, and that a 401 is then reported as
// refused credentials without a retry.
func TestToken_Anonymous(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Authorization header")
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	_, err := liveClient(server, 3, ClientOptions{}).FetchWindow(context.Background(), domain.LiveResource{Name: "laps"}, 1, domain.LiveWindow{})
	if !errors.Is(err, domain.ErrOpenF1Credentials) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
