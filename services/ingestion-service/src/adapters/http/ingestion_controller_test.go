/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_controller_test.go - Package httpadapter source file for services/ingestion-service/src/adapters/http.
##
*/

package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/ingestion-service/src/core/domain"
)

// fakeIngestionUseCase implements ports.OpenF1IngestionUseCase for controller-level tests.
type fakeIngestionUseCase struct {
	result domain.OpenF1IngestionResult
	err    error
}

func (f *fakeIngestionUseCase) Execute(ctx context.Context, request domain.OpenF1IngestionRequest) (domain.OpenF1IngestionResult, error) {
	return f.result, f.err
}

// apiErrorEnvelope mirrors shared/apierror's client-facing JSON shape.
type apiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

// TestIngestionController_TriggerOpenF1Ingestion_ErrorEnvelope is this service's Criterion 6
// representative endpoint. ingestion-service's error surface doesn't naturally include a 404 (it
// has no "identified resource" GET route) or a plain 500 (classifyIngestionError only ever
// returns 400/502/504, by design - see ingestion.controller.go) - so this asserts its actual
// contract (400 malformed body, 400 unsupported resource, 502 upstream failure) through the same
// real apierror envelope, and proves no raw technical error text leaks into the response body.
func TestIngestionController_TriggerOpenF1Ingestion_ErrorEnvelope(t *testing.T) {
	t.Run("malformed JSON body -> 400", func(t *testing.T) {
		controller := NewIngestionController(&fakeIngestionUseCase{})
		req := httptest.NewRequest(http.MethodPost, "/ingest/openf1", bytes.NewReader([]byte("{not-json")))
		rec := httptest.NewRecorder()

		controller.TriggerOpenF1Ingestion(rec, req)

		assertEnvelope(t, rec, http.StatusBadRequest, "VALIDATION_ERROR", "")
	})

	t.Run("unsupported OpenF1 resource -> 400, no leaked detail", func(t *testing.T) {
		wrapped := fmt.Errorf("map not-a-real-resource: %w %q", domain.ErrUnsupportedResource, "not-a-real-resource")
		controller := NewIngestionController(&fakeIngestionUseCase{err: wrapped})
		body, _ := json.Marshal(domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998, Resources: []string{"not-a-real-resource"}})
		req := httptest.NewRequest(http.MethodPost, "/ingest/openf1", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		controller.TriggerOpenF1Ingestion(rec, req)

		assertEnvelope(t, rec, http.StatusBadRequest, "VALIDATION_ERROR", "")
	})

	t.Run("upstream OpenF1/dispatch failure -> 502, no leaked detail", func(t *testing.T) {
		technicalDetail := "dial tcp 10.0.0.5:443: connect: connection refused"
		controller := NewIngestionController(&fakeIngestionUseCase{err: errors.New(technicalDetail)})
		body, _ := json.Marshal(domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
		req := httptest.NewRequest(http.MethodPost, "/ingest/openf1", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		controller.TriggerOpenF1Ingestion(rec, req)

		assertEnvelope(t, rec, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", technicalDetail)
	})
}

// assertEnvelope decodes the real apierror JSON envelope from rec and asserts the expected
// status/code, that the message is non-empty, and (when mustNotContain is non-empty) that the
// raw technical error text never leaked into the response body.
func assertEnvelope(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string, mustNotContain string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, wantStatus, rec.Body.String())
	}
	var envelope apiErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response body is not the expected {\"error\":{...}} envelope: %v (body: %s)", err, rec.Body.String())
	}
	if envelope.Error.Code != wantCode {
		t.Fatalf("error.code = %q, want %q", envelope.Error.Code, wantCode)
	}
	if envelope.Error.Message == "" {
		t.Fatal("error.message must not be empty")
	}
	if mustNotContain != "" && strings.Contains(rec.Body.String(), mustNotContain) {
		t.Fatalf("response body leaked technical error detail: %s", rec.Body.String())
	}
}
