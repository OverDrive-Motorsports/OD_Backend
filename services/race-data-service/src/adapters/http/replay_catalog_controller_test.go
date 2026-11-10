/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog_controller_test.go - Unit tests for the replay catalog HTTP handler.
##
*/

package httpadapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// fakeReplayCatalogUseCase returns canned replay entries and records the championship code.
type fakeReplayCatalogUseCase struct {
	entries []domain.ReplayCatalogEntry
	err     error
	gotCode string
}

func (f *fakeReplayCatalogUseCase) ListReplays(_ context.Context, code string) ([]domain.ReplayCatalogEntry, error) {
	f.gotCode = code
	return f.entries, f.err
}

func serveReplayCatalog(usecase *fakeReplayCatalogUseCase, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	registerReplayCatalogRoutes(mux, NewReplayCatalogController(usecase))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// TestReplayCatalogController_ListReplays_NormalCase proves the route returns the entries as a
// bare camelCase JSON array with every field of the ticket's contract.
func TestReplayCatalogController_ListReplays_NormalCase(t *testing.T) {
	usecase := &fakeReplayCatalogUseCase{entries: []domain.ReplayCatalogEntry{{
		SessionID: "openf1:session:9472", Name: "Race", Type: "race", StartTime: time.Date(2024, 3, 2, 15, 0, 0, 0, time.UTC),
		SeasonYear: 2024, EventName: "Bahrain Grand Prix", Circuit: "Bahrain International Circuit",
	}}}
	rec := serveReplayCatalog(usecase, "/championships/f1/replays")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || usecase.gotCode != "f1" || !strings.HasPrefix(strings.TrimSpace(body), "[") {
		t.Fatalf("status = %d code=%q body=%s", rec.Code, usecase.gotCode, body)
	}
	for _, want := range []string{`"sessionId":"openf1:session:9472"`, `"name":"Race"`, `"startTime":"2024-03-02T15:00:00Z"`, `"seasonYear":2024`, `"eventName":"Bahrain Grand Prix"`, `"circuit":"Bahrain International Circuit"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %s in %s", want, body)
		}
	}
}

// TestReplayCatalogController_ListReplays_StatusCodes proves `[]` for no replay, 404 for an
// unknown championship, 502 for championship-service failures, 500 for a store failure, and
// that no internal error detail leaks into the response body.
func TestReplayCatalogController_ListReplays_StatusCodes(t *testing.T) {
	if rec := serveReplayCatalog(&fakeReplayCatalogUseCase{entries: []domain.ReplayCatalogEntry{}}, "/championships/f1/replays"); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("no replay: status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := serveReplayCatalog(&fakeReplayCatalogUseCase{}, "/championships/nope/replays"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "CHAMPIONSHIP_NOT_FOUND") {
		t.Fatalf("unknown championship: status = %d body=%s", rec.Code, rec.Body.String())
	}
	upstream := fmt.Errorf("%w: %w", domain.ErrChampionshipUnavailable, errors.New("dial tcp 10.0.0.3:3003: refused"))
	if rec := serveReplayCatalog(&fakeReplayCatalogUseCase{err: upstream}, "/championships/f1/replays"); rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "10.0.0.3") {
		t.Fatalf("upstream failure: status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := serveReplayCatalog(&fakeReplayCatalogUseCase{err: errors.New("pq: relation missing")}, "/championships/f1/replays"); rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "pq:") {
		t.Fatalf("store failure: status = %d body=%s", rec.Code, rec.Body.String())
	}
}
