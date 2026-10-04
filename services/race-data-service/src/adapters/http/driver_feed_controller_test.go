/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed_controller_test.go - HTTP tests for GET/PUT /sessions/{sessionId}/drivers/{driverNumber}/broadcast.
##
*/

package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/race-data-service/src/core/domain"
)

// fakeDriverFeedUseCase returns canned results and records the arguments it received.
type fakeDriverFeedUseCase struct {
	result          *domain.DriverBroadcast
	err             error
	called          bool
	gotSessionID    string
	gotDriverNumber int
	gotFeeds        []domain.Feed
}

func (f *fakeDriverFeedUseCase) GetDriverFeeds(_ context.Context, sessionID string, driverNumber int) (*domain.DriverBroadcast, error) {
	f.called = true
	f.gotSessionID = sessionID
	f.gotDriverNumber = driverNumber
	return f.result, f.err
}

func (f *fakeDriverFeedUseCase) ReplaceDriverFeeds(_ context.Context, sessionID string, driverNumber int, feeds []domain.Feed) (*domain.DriverBroadcast, error) {
	f.called = true
	f.gotSessionID = sessionID
	f.gotDriverNumber = driverNumber
	f.gotFeeds = feeds
	return f.result, f.err
}

type driverFeedErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

func putDriverBroadcast(t *testing.T, controller *DriverFeedController, sessionID, driverNumber, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/sessions/"+sessionID+"/drivers/"+driverNumber+"/broadcast", strings.NewReader(body))
	req.SetPathValue("sessionId", sessionID)
	req.SetPathValue("driverNumber", driverNumber)
	rec := httptest.NewRecorder()
	controller.PutDriverBroadcast(rec, req)
	return rec
}

func decodeDriverFeedEnvelope(t *testing.T, rec *httptest.ResponseRecorder) driverFeedErrorEnvelope {
	t.Helper()
	var env driverFeedErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body: %s)", err, rec.Body.String())
	}
	return env
}

// TestDriverFeedController_GetDriverBroadcast_NormalCase proves the read handler emits the
// camelCase `{sessionId, driverNumber, feeds}` payload with `[]` (never null) when empty, and
// forwards the parsed path parameters.
func TestDriverFeedController_GetDriverBroadcast_NormalCase(t *testing.T) {
	usecase := &fakeDriverFeedUseCase{result: &domain.DriverBroadcast{SessionID: "s1", DriverNumber: 44, Feeds: []domain.Feed{}}}
	rec := httptest.NewRecorder()
	NewDriverFeedController(usecase).GetDriverBroadcast(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/44/broadcast", "s1", "44"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != `{"sessionId":"s1","driverNumber":44,"feeds":[]}` {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if usecase.gotSessionID != "s1" || usecase.gotDriverNumber != 44 {
		t.Fatalf("expected path params forwarded, got %q/%d", usecase.gotSessionID, usecase.gotDriverNumber)
	}
}

// TestDriverFeedController_GetDriverBroadcast_Errors proves each usecase sentinel maps to its
// documented status/code (404 SESSION, 404 DRIVER, 502 upstream, 500 generic without leaking
// detail) and that a non-numeric driverNumber is a 400 before the usecase is called.
func TestDriverFeedController_GetDriverBroadcast_Errors(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		status   int
		code     string
		leakText string
	}{
		{"unknown session", domain.ErrSessionNotFound, http.StatusNotFound, "SESSION_NOT_FOUND", ""},
		{"unknown driver", domain.ErrDriverNotFound, http.StatusNotFound, "DRIVER_NOT_FOUND", ""},
		{"championship down", fmt.Errorf("%w: dial tcp refused", domain.ErrChampionshipUnavailable), http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "dial tcp"},
		{"repository failure", errors.New("pg: relation missing"), http.StatusInternalServerError, "INTERNAL_ERROR", "relation missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewDriverFeedController(&fakeDriverFeedUseCase{err: tc.err}).GetDriverBroadcast(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/44/broadcast", "s1", "44"))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.status, rec.Body.String())
			}
			env := decodeDriverFeedEnvelope(t, rec)
			if env.Error.Code != tc.code || env.Error.Status != tc.status {
				t.Fatalf("unexpected envelope: %+v", env)
			}
			if tc.leakText != "" && strings.Contains(env.Error.Message, tc.leakText) {
				t.Fatalf("technical detail leaked to the client: %q", env.Error.Message)
			}
		})
	}

	t.Run("invalid driverNumber -> 400", func(t *testing.T) {
		usecase := &fakeDriverFeedUseCase{}
		rec := httptest.NewRecorder()
		NewDriverFeedController(usecase).GetDriverBroadcast(rec, withDriverPath(http.MethodGet, "/sessions/s1/drivers/abc/broadcast", "s1", "abc"))
		if rec.Code != http.StatusBadRequest || usecase.called {
			t.Fatalf("status = %d called=%v (body: %s)", rec.Code, usecase.called, rec.Body.String())
		}
	})
}

// TestDriverFeedController_PutDriverBroadcast_NormalCase proves a valid body is decoded into
// typed feeds, forwarded with both path identifiers, and the updated payload is echoed back.
func TestDriverFeedController_PutDriverBroadcast_NormalCase(t *testing.T) {
	feeds := []domain.Feed{{Provider: "f1tv", ContentID: "1000005432", ChannelID: "1044", Label: "Onboard"}}
	usecase := &fakeDriverFeedUseCase{result: &domain.DriverBroadcast{SessionID: "s1", DriverNumber: 44, Feeds: feeds}}
	rec := putDriverBroadcast(t, NewDriverFeedController(usecase), "s1", "44", `{"feeds":[{"provider":"f1tv","contentId":"1000005432","channelId":"1044","label":"Onboard"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if usecase.gotSessionID != "s1" || usecase.gotDriverNumber != 44 || len(usecase.gotFeeds) != 1 || usecase.gotFeeds[0] != feeds[0] {
		t.Fatalf("unexpected usecase call: %+v", usecase)
	}
	var got domain.DriverBroadcast
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.SessionID != "s1" || got.DriverNumber != 44 || len(got.Feeds) != 1 || got.Feeds[0].ChannelID != "1044" {
		t.Fatalf("unexpected response: %+v err=%v body=%s", got, err, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "driver_number") || strings.Contains(rec.Body.String(), "session_id") {
		t.Fatalf("expected camelCase keys, got %s", rec.Body.String())
	}
}

// TestDriverFeedController_PutDriverBroadcast_InvalidBody proves malformed JSON, a missing or
// null "feeds" key, unknown fields and trailing data are rejected with a 400 envelope before
// the usecase is called.
func TestDriverFeedController_PutDriverBroadcast_InvalidBody(t *testing.T) {
	for name, body := range map[string]string{
		"malformed json":     `{"feeds": [`,
		"missing feeds":      `{}`,
		"null feeds":         `{"feeds": null}`,
		"unknown field":      `{"feeds": [], "token": "secret"}`,
		"unknown feed field": `{"feeds": [{"provider":"hls","url":"https://a.b/c.m3u8","manifest":"x"}]}`,
		"feeds not an array": `{"feeds": {"provider":"hls"}}`,
		"trailing data":      `{"feeds": []} []`,
	} {
		t.Run(name, func(t *testing.T) {
			usecase := &fakeDriverFeedUseCase{result: &domain.DriverBroadcast{}}
			rec := putDriverBroadcast(t, NewDriverFeedController(usecase), "s1", "44", body)
			if rec.Code != http.StatusBadRequest || usecase.called {
				t.Fatalf("status = %d called=%v (body: %s)", rec.Code, usecase.called, rec.Body.String())
			}
			if env := decodeDriverFeedEnvelope(t, rec); env.Error.Code != "VALIDATION_ERROR" {
				t.Fatalf("unexpected envelope: %+v", env)
			}
		})
	}
}

// TestDriverFeedController_PutDriverBroadcast_Errors proves the usecase sentinels map to 400
// (ErrInvalidFeed, message naming index+field surfaced), 404 SESSION / DRIVER, 502 and 500.
func TestDriverFeedController_PutDriverBroadcast_Errors(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{"invalid feed", fmt.Errorf("%w: feeds[0].url must not be set for provider f1tv", domain.ErrInvalidFeed), http.StatusBadRequest, "VALIDATION_ERROR", "feeds[0].url"},
		{"unknown session", domain.ErrSessionNotFound, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found"},
		{"unknown driver", domain.ErrDriverNotFound, http.StatusNotFound, "DRIVER_NOT_FOUND", "driver not found"},
		{"championship down", fmt.Errorf("%w: dial tcp refused", domain.ErrChampionshipUnavailable), http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "championship-service"},
		{"repository failure", errors.New("pg: relation missing"), http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store driver feeds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := putDriverBroadcast(t, NewDriverFeedController(&fakeDriverFeedUseCase{err: tc.err}), "s1", "44", `{"feeds":[]}`)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.status, rec.Body.String())
			}
			env := decodeDriverFeedEnvelope(t, rec)
			if env.Error.Code != tc.code || !strings.Contains(env.Error.Message, tc.message) {
				t.Fatalf("unexpected envelope: %+v", env)
			}
			if strings.Contains(env.Error.Message, "relation missing") || strings.Contains(env.Error.Message, "dial tcp") {
				t.Fatalf("technical detail leaked to the client: %q", env.Error.Message)
			}
		})
	}

	t.Run("invalid driverNumber -> 400 before body decoding", func(t *testing.T) {
		usecase := &fakeDriverFeedUseCase{}
		rec := putDriverBroadcast(t, NewDriverFeedController(usecase), "s1", "0", `{"feeds":[]}`)
		if rec.Code != http.StatusBadRequest || usecase.called {
			t.Fatalf("status = %d called=%v (body: %s)", rec.Code, usecase.called, rec.Body.String())
		}
	})
}

// TestNewRouter_DriverBroadcastRoutes proves GET and PUT are both registered on the driver
// broadcast path (reaching the feed controller, not the generic /{dataset} catch-all), that
// other methods 405, and that the PUT body limit rejects an oversized payload.
func TestNewRouter_DriverBroadcastRoutes(t *testing.T) {
	usecase := &fakeDriverFeedUseCase{result: &domain.DriverBroadcast{SessionID: "s1", DriverNumber: 44, Feeds: []domain.Feed{}}}
	router := NewRouter(
		NewHealthController(&fakeHealthUseCase{}),
		NewIngestionController(&fakeIngestionBatchUseCase{}),
		NewSessionController(&fakeSessionQueryUseCase{}),
		NewRaceLiveController(&fakeRaceLiveUseCase{}),
		NewTelemetryController(&fakeTelemetryUseCase{}),
		NewRaceReplayStreamController(&fakeRaceReplayUseCase{}),
		NewDriverFeedController(usecase),
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/sessions/s1/drivers/44/broadcast", strings.NewReader(`{"feeds":[]}`)))
	if rec.Code != http.StatusOK || usecase.gotSessionID != "s1" || usecase.gotDriverNumber != 44 {
		t.Fatalf("PUT: status = %d usecase=%+v body=%s", rec.Code, usecase, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sessions/s1/drivers/44/broadcast", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"feeds":[]`) {
		t.Fatalf("GET: status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/sessions/s1/drivers/44/broadcast", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE: status = %d, want 405", rec.Code)
	}

	huge := fmt.Sprintf(`{"feeds":[{"provider":"hls","url":"https://cdn.example.com/%s.m3u8"}]}`, strings.Repeat("a", int(feedListBodyLimitBytes)))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/sessions/s1/drivers/44/broadcast", strings.NewReader(huge)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized PUT: status = %d, want 400", rec.Code)
	}
}
