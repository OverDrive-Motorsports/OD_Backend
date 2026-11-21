/**
##
## OverDrive 2026
## All Technical rights reserved
##
## internal_token_test.go - Tests the service-to-service token on the internal ingestion endpoint.
##
*/

package httpadapter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testInternalToken stands for INTERNAL_INGESTION_TOKEN.
const testInternalToken = "internal-token-long-enough-for-the-check"

// TestInternalIngestionEndpoint_RequiresToken proves POST /internal/ingestion/batches, through
// the real router, answers 401 without the token or with a wrong one - without reaching the
// usecase - and accepts the batch with the right token.
func TestInternalIngestionEndpoint_RequiresToken(t *testing.T) {
	for name, token := range map[string]string{"missing": "", "wrong": "internal-token-long-enough-but-not-right"} {
		usecase := &fakeIngestionBatchUseCase{}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/ingestion/batches", strings.NewReader(`{"batch_id":"b1"}`))
		if token != "" {
			req.Header.Set(InternalTokenHeader, token)
		}
		NewRouter(NewHealthController(nil), NewIngestionController(usecase), NewSessionController(nil), NewRaceLiveController(nil), NewTelemetryController(nil), NewRaceReplayStreamController(nil), NewReplayCatalogController(nil), testInternalToken).ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
			t.Fatalf("%s token: status = %d body=%s, want 401 UNAUTHORIZED", name, rec.Code, rec.Body.String())
		}
		if usecase.got.BatchID != "" {
			t.Fatalf("%s token: the usecase must not be reached", name)
		}
	}

	usecase := &fakeIngestionBatchUseCase{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/ingestion/batches", strings.NewReader(`{"batch_id":"b1"}`))
	req.Header.Set(InternalTokenHeader, testInternalToken)
	NewRouter(NewHealthController(nil), NewIngestionController(usecase), NewSessionController(nil), NewRaceLiveController(nil), NewTelemetryController(nil), NewRaceReplayStreamController(nil), NewReplayCatalogController(nil), testInternalToken).ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized || !(usecase.got.BatchID == "b1") {
		t.Fatalf("valid token: status = %d body=%s, want the batch forwarded to the usecase", rec.Code, rec.Body.String())
	}
}

// TestValidateInternalToken proves the service refuses to start with a missing or short
// INTERNAL_INGESTION_TOKEN and accepts a 32-byte one.
func TestValidateInternalToken(t *testing.T) {
	for _, token := range []string{"", "short", strings.Repeat("a", MinInternalTokenBytes-1)} {
		if err := ValidateInternalToken(token); !errors.Is(err, ErrWeakInternalToken) {
			t.Fatalf("%q: expected ErrWeakInternalToken, got %v", token, err)
		}
	}
	if err := ValidateInternalToken(strings.Repeat("a", MinInternalTokenBytes)); err != nil {
		t.Fatalf("expected a 32-byte token to be accepted, got %v", err)
	}
}
