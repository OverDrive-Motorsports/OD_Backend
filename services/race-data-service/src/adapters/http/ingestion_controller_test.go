/**
##
## OverDrive 2026
## All Technical rights reserved
##
## ingestion_controller_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
##
*/

package httpadapter

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/race-data-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

// fakeIngestionBatchUseCase implements ports.IngestionBatchUseCase.
type fakeIngestionBatchUseCase struct {
	ack error // non-nil to force Execute to fail
	got contracts.Batch
}

func (f *fakeIngestionBatchUseCase) Execute(ctx context.Context, batch contracts.Batch) (domain.IngestionBatchAck, error) {
	f.got = batch
	if f.ack != nil {
		return domain.IngestionBatchAck{}, f.ack
	}
	return domain.NewIngestionBatchAck("race-data-service", batch, batch.TriggeredAt), nil
}

// TestIngestionController_ReceiveBatch_NormalCase proves a well-formed batch body decodes and is
// forwarded to the usecase, writing a 202 with the acknowledgement.
func TestIngestionController_ReceiveBatch_NormalCase(t *testing.T) {
	uc := &fakeIngestionBatchUseCase{}
	c := NewIngestionController(uc)

	body := `{"batch_id":"b1","provider":"openf1","datasets":[{"name":"laps","target_service":"race-data-service","row_count":1,"rows":[{"lap_number":1}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/ingestion/batches", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	c.ReceiveBatch(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body: %s)", rec.Code, rec.Body.String())
	}
	if uc.got.BatchID != "b1" {
		t.Fatalf("expected the decoded batch to be forwarded to the usecase, got %+v", uc.got)
	}
}

// TestIngestionController_ReceiveBatch_InvalidJSON proves a malformed body is rejected with 400
// before the usecase is ever called.
func TestIngestionController_ReceiveBatch_InvalidJSON(t *testing.T) {
	c := NewIngestionController(&fakeIngestionBatchUseCase{})
	req := httptest.NewRequest(http.MethodPost, "/ingestion/batches", bytes.NewBufferString("not-json"))
	rec := httptest.NewRecorder()

	c.ReceiveBatch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestIngestionController_ReceiveBatch_UsecaseValidationFailure proves a usecase-level validation
// failure (e.g. missing batch_id) is reported as 400, and - per the apierror migration's
// "Message vs Err" rule - the raw technical error text is never leaked into the client-facing
// message.
func TestIngestionController_ReceiveBatch_UsecaseValidationFailure(t *testing.T) {
	technicalDetail := "pq: relation \"race_lap\" does not exist"
	uc := &fakeIngestionBatchUseCase{ack: errors.New(technicalDetail)}
	c := NewIngestionController(uc)

	body := `{"batch_id":"b1","provider":"openf1","datasets":[{"name":"laps","target_service":"race-data-service"}]}`
	req := httptest.NewRequest(http.MethodPost, "/ingestion/batches", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	c.ReceiveBatch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), technicalDetail) {
		t.Fatalf("response body leaked technical error detail: %s", rec.Body.String())
	}
}
