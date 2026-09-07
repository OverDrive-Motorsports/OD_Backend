/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## session_controller_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/race-data-service/src/core/domain"
)

// fakeSessionQueryUseCase implements ports.SessionQueryUseCase. Only GetSessionDataset is
// exercised by these tests; every other method is unused stub surface required to satisfy the
// interface.
type fakeSessionQueryUseCase struct {
	datasetResult map[string]any
	datasetErr    error
}

func (f *fakeSessionQueryUseCase) ListChampionships(ctx context.Context) (any, error) { return nil, nil }
func (f *fakeSessionQueryUseCase) ListChampionshipEvents(ctx context.Context, code string) (any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetEvent(ctx context.Context, eventID string) (any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) ListEventSessions(ctx context.Context, eventID string) (any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetSession(ctx context.Context, sessionID string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetSessionMetadata(ctx context.Context, sessionID string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) ListSessionDrivers(ctx context.Context, sessionID string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) ListSessionTeams(ctx context.Context, sessionID string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (map[string]any, error) {
	return f.datasetResult, f.datasetErr
}
func (f *fakeSessionQueryUseCase) GetSessionRaceStandings(ctx context.Context, sessionID string) (any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetSessionBroadcast(ctx context.Context, sessionID string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetDriverProfile(ctx context.Context, sessionID string, driverNumber int) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSessionQueryUseCase) GetSessionFacts(ctx context.Context, sessionID string) (map[string]any, error) {
	return nil, nil
}

// apiErrorEnvelope mirrors shared/apierror's client-facing JSON shape, used to decode responses
// in these tests without importing apierror's internal (unexported) responseBody type.
type apiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

// TestSessionController_GetSessionDataset_ErrorEnvelope is this service's Criterion 6
// representative endpoint: it exercises all three of the shared 404/400/500 error contract cases
// through the real apierror envelope (shared/apierror), asserting no raw technical error text
// (e.g. a Prisma/SQL error string) ever reaches the response body - only the fixed client-facing
// Message, never Err.Error().
func TestSessionController_GetSessionDataset_ErrorEnvelope(t *testing.T) {
	technicalDetail := "pq: relation \"race_lap\" does not exist, conn=17ffae2"

	tests := []struct {
		name           string
		usecase        *fakeSessionQueryUseCase
		wantStatus     int
		wantCode       string
		mustNotContain string
	}{
		{
			name:       "unknown session -> 404",
			usecase:    &fakeSessionQueryUseCase{datasetResult: nil, datasetErr: nil},
			wantStatus: http.StatusNotFound,
			wantCode:   "SESSION_NOT_FOUND",
		},
		{
			name:       "unknown dataset name -> 400",
			usecase:    &fakeSessionQueryUseCase{datasetErr: domain.ErrUnknownDataset},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:           "repository failure -> 500, no leaked technical detail",
			usecase:        &fakeSessionQueryUseCase{datasetErr: errors.New(technicalDetail)},
			wantStatus:     http.StatusInternalServerError,
			wantCode:       "INTERNAL_ERROR",
			mustNotContain: technicalDetail,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := NewSessionController(tc.usecase)

			req := httptest.NewRequest(http.MethodGet, "/sessions/s1/datasets/laps", nil)
			req.SetPathValue("sessionId", "s1")
			req.SetPathValue("dataset", "laps")
			rec := httptest.NewRecorder()

			controller.GetSessionDataset(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}

			var envelope apiErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("response body is not the expected {\"error\":{...}} envelope: %v (body: %s)", err, rec.Body.String())
			}
			if envelope.Error.Code != tc.wantCode {
				t.Fatalf("error.code = %q, want %q", envelope.Error.Code, tc.wantCode)
			}
			if envelope.Error.Status != tc.wantStatus {
				t.Fatalf("error.status = %d, want %d", envelope.Error.Status, tc.wantStatus)
			}
			if envelope.Error.Message == "" {
				t.Fatal("error.message must not be empty")
			}
			if tc.mustNotContain != "" && strings.Contains(rec.Body.String(), tc.mustNotContain) {
				t.Fatalf("response body leaked technical error detail: %s", rec.Body.String())
			}
		})
	}
}
