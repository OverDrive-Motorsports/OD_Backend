/**
##
## OverDrive 2026
## All Technical rights reserved
##
## dispatcher_test.go - Package httpdispatcher source file for services/ingestion-service/src/adapters/dispatch/http.
##
*/

package httpdispatcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	contracts "overdrive/shared/contracts/ingestion"
)

// testInternalToken stands for INTERNAL_INGESTION_TOKEN.
const testInternalToken = "internal-token-long-enough-for-the-check"

// TestDispatcher_Dispatch_Success proves a successful downstream response is decoded into the
// DispatchAck the caller expects, that the batch is sent to the correct
// /internal/ingestion/batches path for the requested service, and that the internal token
// travels in X-Internal-Token (the receiving services reject batches without it).
func TestDispatcher_Dispatch_Success(t *testing.T) {
	var gotPath, gotToken string
	var gotBatch contracts.Batch

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Internal-Token")
		_ = json.NewDecoder(r.Body).Decode(&gotBatch)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contracts.DispatchAck{Service: "race-data-service", DatasetCount: 1, TotalRowCount: 3})
	}))
	defer server.Close()

	d := NewDispatcher(server.URL, server.URL, testInternalToken, time.Second)
	ack, err := d.Dispatch(context.Background(), "race-data-service", contracts.Batch{BatchID: "b1", Datasets: []contracts.Dataset{{Name: "laps", RowCount: 3}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ack.Service != "race-data-service" || ack.DatasetCount != 1 || ack.TotalRowCount != 3 {
		t.Fatalf("unexpected ack: %+v", ack)
	}
	if gotPath != "/internal/ingestion/batches" {
		t.Fatalf("expected the batches endpoint to be hit, got path %q", gotPath)
	}
	if gotBatch.BatchID != "b1" {
		t.Fatalf("expected the batch body to be forwarded, got %+v", gotBatch)
	}
	if gotToken != testInternalToken {
		t.Fatalf("expected the internal token in X-Internal-Token, got %q", gotToken)
	}
}

// TestDispatcher_Dispatch_UnknownService proves requesting dispatch to a service with no
// configured endpoint fails fast with a clear error, instead of silently doing nothing or
// panicking on a lookup miss.
func TestDispatcher_Dispatch_UnknownService(t *testing.T) {
	d := NewDispatcher("http://race-data", "http://championship", testInternalToken, time.Second)
	_, err := d.Dispatch(context.Background(), "not-a-real-service", contracts.Batch{})
	if err == nil {
		t.Fatal("expected an error for an unconfigured service")
	}
}

// TestDispatcher_Dispatch_UpstreamErrorStatusPropagates proves a 4xx/5xx downstream response is
// turned into a Go error carrying the status code and body, rather than being treated as success.
func TestDispatcher_Dispatch_UpstreamErrorStatusPropagates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"invalid batch"}}`))
	}))
	defer server.Close()

	d := NewDispatcher(server.URL, server.URL, testInternalToken, time.Second)
	_, err := d.Dispatch(context.Background(), "race-data-service", contracts.Batch{})
	if err == nil {
		t.Fatal("expected an error for a 400 downstream response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected the error to mention the status code, got %v", err)
	}
}
