/**
##
## OverDrive 2026
## All Technical rights reserved
##
## client_test.go - Unit tests for the OpenF1 HTTP client behavior.
##
*/

package openf1_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"overdrive/internal/providers/openf1"
	"overdrive/tests/internal/mocks"
)

// TestClientGetSuccess verifies the client builds the request URL and decodes JSON arrays.
func TestClientGetSuccess(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = mocks.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/drivers" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("session_key") != "9693" {
			t.Fatalf("unexpected session_key: %s", r.URL.Query().Get("session_key"))
		}
		return mocks.JSONResponse(http.StatusOK, `[{"driver_number":63,"full_name":"George Russell"}]`), nil
	})
	defer func() { http.DefaultTransport = oldTransport }()

	client := openf1.NewClient("http://openf1.test/v1", time.Second, 0, 0, 0)
	rows, err := client.Get(context.Background(), "drivers", map[string]string{"session_key": "9693"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("unexpected row count: %d", len(rows))
	}
	if rows[0]["driver_number"].(float64) != 63 {
		t.Fatalf("unexpected driver number: %#v", rows[0]["driver_number"])
	}
}

// TestClientGetHTTPError verifies provider 4xx responses are wrapped as HTTPError.
func TestClientGetHTTPError(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = mocks.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return mocks.JSONResponse(http.StatusNotFound, `{"detail":"No results found."}`), nil
	})
	defer func() { http.DefaultTransport = oldTransport }()

	client := openf1.NewClient("http://openf1.test", time.Second, 0, 0, 0)
	_, err := client.Get(context.Background(), "laps", map[string]string{"session_key": "9693"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var httpErr *openf1.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected status code: %d", httpErr.StatusCode)
	}
	if httpErr.Endpoint != "laps" {
		t.Fatalf("unexpected endpoint: %s", httpErr.Endpoint)
	}
}
