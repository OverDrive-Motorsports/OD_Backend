/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_feed_controller_test.go - HTTP tests for GET/PUT /sessions/{sessionId}/broadcast.
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

	"overdrive/services/championship-service/src/core/domain"
	"overdrive/services/championship-service/src/core/ports"
	"overdrive/services/championship-service/src/core/usecases"
)

// fakeSessionFeedUseCase returns canned results and records the feeds it received.
type fakeSessionFeedUseCase struct {
	result   *domain.SessionBroadcast
	err      error
	gotID    string
	gotFeeds []domain.Feed
	called   bool
}

func (f *fakeSessionFeedUseCase) GetSessionFeeds(_ context.Context, sessionID string) (*domain.SessionBroadcast, error) {
	f.gotID = sessionID
	return f.result, f.err
}

func (f *fakeSessionFeedUseCase) ReplaceSessionFeeds(_ context.Context, sessionID string, feeds []domain.Feed) (*domain.SessionBroadcast, error) {
	f.called = true
	f.gotID = sessionID
	f.gotFeeds = feeds
	return f.result, f.err
}

type feedErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

func putBroadcast(t *testing.T, controller *SessionFeedController, sessionID string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/sessions/"+sessionID+"/broadcast", strings.NewReader(body))
	req.SetPathValue("sessionId", sessionID)
	rec := httptest.NewRecorder()
	controller.PutSessionBroadcast(rec, req)
	return rec
}

// TestSessionFeedController_GetSessionBroadcast proves the read handler returns the
// `{sessionId, feeds}` payload with feeds serialised as `[]` (never null) when empty, and the
// 404 / 500 branches.
func TestSessionFeedController_GetSessionBroadcast(t *testing.T) {
	t.Run("normal case with empty list", func(t *testing.T) {
		usecase := &fakeSessionFeedUseCase{result: &domain.SessionBroadcast{SessionID: "s1", Feeds: []domain.Feed{}}}
		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/broadcast", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		NewSessionFeedController(usecase).GetSessionBroadcast(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if strings.TrimSpace(rec.Body.String()) != `{"sessionId":"s1","feeds":[]}` {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
		if usecase.gotID != "s1" {
			t.Fatalf("expected sessionId forwarded, got %q", usecase.gotID)
		}
	})

	t.Run("unknown session -> 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions/nope/broadcast", nil)
		req.SetPathValue("sessionId", "nope")
		rec := httptest.NewRecorder()
		NewSessionFeedController(&fakeSessionFeedUseCase{}).GetSessionBroadcast(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("repository failure -> 500 without leaking detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/broadcast", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		NewSessionFeedController(&fakeSessionFeedUseCase{err: errors.New("pg: connection refused")}).GetSessionBroadcast(rec, req)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "connection refused") {
			t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
		}
	})
}

// TestSessionFeedController_PutSessionBroadcast_NormalCase proves a valid body is decoded into
// typed feeds, forwarded to the usecase with the path sessionId, and the updated payload is
// echoed back with camelCase keys.
func TestSessionFeedController_PutSessionBroadcast_NormalCase(t *testing.T) {
	feeds := []domain.Feed{
		{Provider: "f1tv", ContentID: "1000005432", ChannelID: "1017", Label: "Onboard"},
		{Provider: "youtube", URL: "https://www.youtube.com/watch?v=abc"},
	}
	usecase := &fakeSessionFeedUseCase{result: &domain.SessionBroadcast{SessionID: "s1", Feeds: feeds}}
	body := `{"feeds":[{"provider":"f1tv","contentId":"1000005432","channelId":"1017","label":"Onboard"},{"provider":"youtube","url":"https://www.youtube.com/watch?v=abc"}]}`
	rec := putBroadcast(t, NewSessionFeedController(usecase), "s1", body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if usecase.gotID != "s1" || len(usecase.gotFeeds) != 2 || usecase.gotFeeds[0] != feeds[0] || usecase.gotFeeds[1] != feeds[1] {
		t.Fatalf("unexpected usecase call: id=%q feeds=%+v", usecase.gotID, usecase.gotFeeds)
	}
	var got domain.SessionBroadcast
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, rec.Body.String())
	}
	if got.SessionID != "s1" || len(got.Feeds) != 2 || got.Feeds[0].ContentID != "1000005432" {
		t.Fatalf("unexpected response: %+v", got)
	}
	if !strings.Contains(rec.Body.String(), `"contentId"`) || strings.Contains(rec.Body.String(), `"content_id"`) {
		t.Fatalf("expected camelCase keys, got %s", rec.Body.String())
	}
}

// TestSessionFeedController_PutSessionBroadcast_EmptyListClears proves `{"feeds":[]}` is a
// valid replacement (clears the list) and reaches the usecase with a non-nil empty slice.
func TestSessionFeedController_PutSessionBroadcast_EmptyListClears(t *testing.T) {
	usecase := &fakeSessionFeedUseCase{result: &domain.SessionBroadcast{SessionID: "s1", Feeds: []domain.Feed{}}}
	rec := putBroadcast(t, NewSessionFeedController(usecase), "s1", `{"feeds":[]}`)
	if rec.Code != http.StatusOK || !usecase.called || usecase.gotFeeds == nil || len(usecase.gotFeeds) != 0 {
		t.Fatalf("status = %d called=%v feeds=%#v body=%s", rec.Code, usecase.called, usecase.gotFeeds, rec.Body.String())
	}
}

// TestSessionFeedController_PutSessionBroadcast_InvalidBody proves malformed JSON, a missing
// "feeds" key, an unknown top-level field, a non-array feeds value and trailing data are all
// rejected with a 400 VALIDATION envelope before the usecase is ever called.
func TestSessionFeedController_PutSessionBroadcast_InvalidBody(t *testing.T) {
	for name, body := range map[string]string{
		"malformed json":      `{"feeds": [`,
		"missing feeds":       `{}`,
		"null feeds":          `{"feeds": null}`,
		"unknown field":       `{"feeds": [], "token": "secret"}`,
		"feeds not an array":  `{"feeds": "https://youtu.be/a"}`,
		"trailing data":       `{"feeds": []}{"feeds": []}`,
		"empty body":          ``,
		"unknown feed field":  `{"feeds": [{"provider":"hls","url":"https://a.b/c.m3u8","token":"x"}]}`,
		"wrong provider type": `{"feeds": [{"provider": 1}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			usecase := &fakeSessionFeedUseCase{result: &domain.SessionBroadcast{SessionID: "s1"}}
			rec := putBroadcast(t, NewSessionFeedController(usecase), "s1", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
			}
			if usecase.called {
				t.Fatal("usecase must not be called for an invalid body")
			}
			var env feedErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != "VALIDATION_ERROR" || env.Error.Status != 400 {
				t.Fatalf("unexpected envelope: %s (err=%v)", rec.Body.String(), err)
			}
		})
	}
}

// TestSessionFeedController_PutSessionBroadcast_ValidationErrorIs400 proves a
// domain.ErrInvalidFeed from the (real) usecase maps to 400 and the client-facing message names
// the failing index and field, including the f1tv-with-url case the ticket calls out.
func TestSessionFeedController_PutSessionBroadcast_ValidationErrorIs400(t *testing.T) {
	repo := &recordingSessionFeedRepository{}
	controller := NewSessionFeedController(usecases.NewSessionFeedUseCase(repo))
	body := `{"feeds":[{"provider":"hls","url":"https://cdn.example.com/a.m3u8"},{"provider":"f1tv","contentId":"1","channelId":"2","url":"https://f1tv.formula1.com/manifest.mpd?token=abc"}]}`
	rec := putBroadcast(t, controller, "s1", body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	var env feedErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != "VALIDATION_ERROR" || !strings.Contains(env.Error.Message, "feeds[1].url") {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if repo.called {
		t.Fatal("repository must not be reached when validation fails")
	}
}

// TestSessionFeedController_PutSessionBroadcast_UnknownSession proves a nil usecase result
// answers 404 SESSION_NOT_FOUND.
func TestSessionFeedController_PutSessionBroadcast_UnknownSession(t *testing.T) {
	rec := putBroadcast(t, NewSessionFeedController(&fakeSessionFeedUseCase{}), "missing", `{"feeds":[]}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
	var env feedErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != "SESSION_NOT_FOUND" {
		t.Fatalf("unexpected envelope: %s", rec.Body.String())
	}
}

// TestSessionFeedController_PutSessionBroadcast_InternalError proves a non-validation usecase
// error is a 500 whose message never contains the technical detail.
func TestSessionFeedController_PutSessionBroadcast_InternalError(t *testing.T) {
	rec := putBroadcast(t, NewSessionFeedController(&fakeSessionFeedUseCase{err: errors.New("pg: relation missing")}), "s1", `{"feeds":[]}`)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "relation missing") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

// TestNewRouter_SessionBroadcastRoutes proves both methods are registered on
// /sessions/{sessionId}/broadcast (a PUT reaches the feed controller, a GET too), that the PUT
// body limit rejects an oversized payload, and that other methods still 405.
func TestNewRouter_SessionBroadcastRoutes(t *testing.T) {
	usecase := &fakeSessionFeedUseCase{result: &domain.SessionBroadcast{SessionID: "s1", Feeds: []domain.Feed{}}}
	router := NewRouter(
		NewHealthController(&fakeHealthUseCase{}),
		NewIngestionController(&fakeIngestionUseCase{}),
		NewCatalogController(&fakeCatalogUseCase{}),
		NewSessionFeedController(usecase),
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/sessions/s1/broadcast", strings.NewReader(`{"feeds":[]}`)))
	if rec.Code != http.StatusOK || usecase.gotID != "s1" {
		t.Fatalf("PUT: status = %d id=%q body=%s", rec.Code, usecase.gotID, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sessions/s1/broadcast", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/sessions/s1/broadcast", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE: status = %d, want 405", rec.Code)
	}

	huge := fmt.Sprintf(`{"feeds":[{"provider":"hls","url":"https://cdn.example.com/%s.m3u8"}]}`, strings.Repeat("a", int(feedListBodyLimitBytes)))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/sessions/s1/broadcast", strings.NewReader(huge)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized PUT: status = %d, want 400", rec.Code)
	}
}

// recordingSessionFeedRepository is a ports.SessionFeedRepository that only records whether it
// was reached, for tests driving the real usecase through the controller.
type recordingSessionFeedRepository struct {
	called bool
}

var _ ports.SessionFeedRepository = (*recordingSessionFeedRepository)(nil)

func (r *recordingSessionFeedRepository) GetSessionFeeds(context.Context, string) (*domain.SessionBroadcast, error) {
	r.called = true
	return nil, nil
}

func (r *recordingSessionFeedRepository) ReplaceSessionFeeds(_ context.Context, sessionID string, feeds []domain.Feed) (*domain.SessionBroadcast, error) {
	r.called = true
	return &domain.SessionBroadcast{SessionID: sessionID, Feeds: feeds}, nil
}
