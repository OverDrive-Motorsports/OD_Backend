/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live_controller_test.go - Unit tests for the live ingestion routes.
##
*/

package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/ingestion-service/src/core/domain"
)

// fakeLiveUseCase implements ports.LiveIngestionUseCase.
type fakeLiveUseCase struct {
	status  domain.LiveStatus
	started bool
	err     error
	calls   []string
	gotKey  int
}

func (f *fakeLiveUseCase) Start(ctx context.Context, sessionKey int) (domain.LiveStatus, bool, error) {
	f.calls, f.gotKey = append(f.calls, "start"), sessionKey
	if _, bounded := ctx.Deadline(); !bounded {
		return domain.LiveStatus{}, false, errors.New("start must be bounded")
	}
	return f.status, f.started, f.err
}

func (f *fakeLiveUseCase) Stop(ctx context.Context, sessionKey int) (domain.LiveStatus, error) {
	f.calls, f.gotKey = append(f.calls, "stop"), sessionKey
	return f.status, f.err
}

func (f *fakeLiveUseCase) Status(sessionKey int) (domain.LiveStatus, error) {
	f.calls, f.gotKey = append(f.calls, "status"), sessionKey
	return f.status, f.err
}

// serveLive sends one request through the real router.
func serveLive(usecase *fakeLiveUseCase, method string, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	NewRouter(NewHealthController(&fakeHealthUseCase{}), NewIngestionController(&fakeIngestionUseCase{}), NewLiveController(usecase)).
		ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

// TestLiveRoutes proves each route reaches its handler with the session key and answers the
// camelCase status: 202 for a new job, 200 for one already running, a stop or a status.
func TestLiveRoutes(t *testing.T) {
	for _, tc := range []struct {
		method, action string
		started        bool
		want           int
	}{
		{http.MethodPost, "start", true, http.StatusAccepted},
		{http.MethodPost, "start", false, http.StatusOK},
		{http.MethodPost, "stop", false, http.StatusOK},
		{http.MethodGet, "status", false, http.StatusOK},
	} {
		usecase := &fakeLiveUseCase{started: tc.started, status: domain.LiveStatus{SessionKey: 9998, State: domain.LiveRunning, Resources: []domain.LiveResourceStatus{}}}
		rec := serveLive(usecase, tc.method, "/providers/openf1/live/9998/"+tc.action)
		if rec.Code != tc.want || usecase.gotKey != 9998 || len(usecase.calls) != 1 || usecase.calls[0] != tc.action {
			t.Fatalf("%s: status=%d calls=%v key=%d", tc.action, rec.Code, usecase.calls, usecase.gotKey)
		}
		for _, part := range []string{`"sessionKey":9998`, `"state":"running"`, `"resources":[]`, `"endedAtUtc":null`} {
			if !strings.Contains(rec.Body.String(), part) {
				t.Fatalf("%s: body %s misses %s", tc.action, rec.Body.String(), part)
			}
		}
	}
}

// TestLiveRoutes_InvalidSessionKey proves a session key that is not a positive integer is a 400 on
// every route, before the use case.
func TestLiveRoutes_InvalidSessionKey(t *testing.T) {
	for _, key := range []string{"abc", "0", "-1", "1.5", "99999999999999"} {
		for _, route := range []struct{ method, action string }{{http.MethodPost, "start"}, {http.MethodPost, "stop"}, {http.MethodGet, "status"}} {
			usecase := &fakeLiveUseCase{}
			rec := serveLive(usecase, route.method, "/providers/openf1/live/"+key+"/"+route.action)
			if rec.Code != http.StatusBadRequest || len(usecase.calls) != 0 || !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
				t.Fatalf("%s %s: status=%d body=%s", key, route.action, rec.Code, rec.Body.String())
			}
		}
	}
}

// TestLiveRoutes_WrongMethod proves the other methods answer 405.
func TestLiveRoutes_WrongMethod(t *testing.T) {
	for _, route := range []struct{ method, action string }{{http.MethodGet, "start"}, {http.MethodGet, "stop"}, {http.MethodPost, "status"}} {
		if rec := serveLive(&fakeLiveUseCase{}, route.method, "/providers/openf1/live/1/"+route.action); rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s: status=%d", route.method, route.action, rec.Code)
		}
	}
}

// TestLiveRoutes_Errors proves each error is mapped to its status and code on every route, and that
// the technical cause never reaches the client.
func TestLiveRoutes_Errors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
		code string
	}{
		{domain.ErrLiveSessionNotFound, http.StatusNotFound, "OPENF1_SESSION_NOT_FOUND"},
		{domain.ErrLiveJobNotFound, http.StatusNotFound, "LIVE_JOB_NOT_FOUND"},
		{domain.ErrLiveJobLimit, http.StatusConflict, "LIVE_JOB_CONFLICT"},
		{domain.ErrLiveJobStopping, http.StatusConflict, "LIVE_JOB_CONFLICT"},
		{domain.ErrSessionBusy, http.StatusConflict, "SESSION_INGESTION_CONFLICT"},
		{domain.ErrLiveShuttingDown, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{context.DeadlineExceeded, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT"},
		{errors.New("dial tcp 10.0.0.1: refused"), http.StatusBadGateway, "UPSTREAM_UNAVAILABLE"},
	} {
		for _, route := range []struct{ method, action string }{{http.MethodPost, "start"}, {http.MethodPost, "stop"}, {http.MethodGet, "status"}} {
			rec := serveLive(&fakeLiveUseCase{err: tc.err}, route.method, "/providers/openf1/live/1/"+route.action)
			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.code) || strings.Contains(rec.Body.String(), "10.0.0.1") {
				t.Fatalf("%s %v: status=%d body=%s", route.action, tc.err, rec.Code, rec.Body.String())
			}
		}
	}
}

// TestManualIngestion_SessionBusy proves a manual ingestion refused because of a live job is a 409.
func TestManualIngestion_SessionBusy(t *testing.T) {
	rec := httptest.NewRecorder()
	NewIngestionController(&fakeIngestionUseCase{err: domain.ErrSessionBusy}).TriggerOpenF1Ingestion(rec,
		httptest.NewRequest(http.MethodPost, "/providers/openf1/ingestions", strings.NewReader(`{"meeting_key":1,"session_key":2}`)))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "SESSION_INGESTION_CONFLICT") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
