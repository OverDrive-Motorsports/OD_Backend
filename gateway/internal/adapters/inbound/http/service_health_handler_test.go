/**
##
## OverDrive 2026
## All Technical rights reserved
##
## service_health_handler_test.go - Unit tests for ServiceHealthHandler.
##
*/

package httpinbound

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestServiceHealthHandler_UpstreamHealthy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("upstream received path %q, want %q", r.URL.Path, "/health")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	handler := NewServiceHealthHandler("race-data-service", []*url.URL{upstreamURL})

	req := httptest.NewRequest(http.MethodGet, "/health/race-data-service", nil)
	rec := httptest.NewRecorder()

	handler.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServiceHealthHandler_UpstreamUnhealthy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	handler := NewServiceHealthHandler("race-data-service", []*url.URL{upstreamURL})

	req := httptest.NewRequest(http.MethodGet, "/health/race-data-service", nil)
	rec := httptest.NewRecorder()

	handler.HandleHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestServiceHealthHandler_UpstreamUnreachable(t *testing.T) {
	deadURL, err := url.Parse("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	handler := NewServiceHealthHandler("race-data-service", []*url.URL{deadURL})

	req := httptest.NewRequest(http.MethodGet, "/health/race-data-service", nil)
	rec := httptest.NewRecorder()

	handler.HandleHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestServiceHealthHandler_RejectsNonGet(t *testing.T) {
	handler := NewServiceHealthHandler("race-data-service", []*url.URL{{Scheme: "http", Host: "example.invalid"}})

	req := httptest.NewRequest(http.MethodPost, "/health/race-data-service", nil)
	rec := httptest.NewRecorder()

	handler.HandleHealth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestServiceHealthHandler_RoundRobinsAcrossTargets(t *testing.T) {
	makeUpstream := func(status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
	}

	healthy := makeUpstream(http.StatusOK)
	defer healthy.Close()
	unhealthy := makeUpstream(http.StatusInternalServerError)
	defer unhealthy.Close()

	healthyURL, _ := url.Parse(healthy.URL)
	unhealthyURL, _ := url.Parse(unhealthy.URL)

	handler := NewServiceHealthHandler("race-data-service", []*url.URL{healthyURL, unhealthyURL})

	var statuses []int
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health/race-data-service", nil)
		rec := httptest.NewRecorder()
		handler.HandleHealth(rec, req)
		statuses = append(statuses, rec.Code)
	}

	if statuses[0] == statuses[1] {
		t.Errorf("expected alternating results across targets, got %v twice", statuses[0])
	}
}
