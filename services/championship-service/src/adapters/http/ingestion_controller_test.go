/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion_controller_test.go - Package httpadapter source file for services/championship-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/championship-service/src/core/domain"
	contracts "overdrive/shared/contracts/ingestion"
)

// fakeIngestionUseCase implements ports.IngestionBatchUseCase for controller-level tests.
type fakeIngestionUseCase struct {
	ack       domain.IngestionBatchAck
	err       error
	lastBatch contracts.Batch
}

func (f *fakeIngestionUseCase) Execute(ctx context.Context, batch contracts.Batch) (domain.IngestionBatchAck, error) {
	f.lastBatch = batch
	return f.ack, f.err
}

// TestIngestionController_ReceiveBatch_Success proves a well-formed batch is decoded, forwarded
// to the usecase, and acknowledged with a 202 and the usecase's ack payload verbatim.
func TestIngestionController_ReceiveBatch_Success(t *testing.T) {
	usecase := &fakeIngestionUseCase{ack: domain.IngestionBatchAck{
		Service:      "championship-service",
		BatchID:      "batch-1",
		Accepted:     true,
		DatasetCount: 1,
	}}
	controller := NewIngestionController(usecase)

	body, err := json.Marshal(contracts.Batch{
		BatchID: "batch-1",
		Datasets: []contracts.Dataset{
			{Name: "event_catalog", TargetService: "championship-service", RowCount: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingestion", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	controller.ReceiveBatch(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body: %s)", rec.Code, rec.Body.String())
	}
	var got domain.IngestionBatchAck
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v (body: %s)", err, rec.Body.String())
	}
	if got != usecase.ack {
		t.Fatalf("got %+v, want %+v", got, usecase.ack)
	}
	if usecase.lastBatch.BatchID != "batch-1" {
		t.Fatalf("expected the decoded batch to be forwarded to the usecase, got %+v", usecase.lastBatch)
	}
}

// TestIngestionController_ReceiveBatch_MalformedBody proves a body that isn't valid JSON is
// rejected as a 400 before the usecase is ever invoked.
func TestIngestionController_ReceiveBatch_MalformedBody(t *testing.T) {
	usecase := &fakeIngestionUseCase{}
	controller := NewIngestionController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/ingestion", strings.NewReader("not-json"))
	rec := httptest.NewRecorder()

	controller.ReceiveBatch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if usecase.lastBatch.BatchID != "" {
		t.Fatal("expected the usecase not to be invoked for a malformed body")
	}
}
