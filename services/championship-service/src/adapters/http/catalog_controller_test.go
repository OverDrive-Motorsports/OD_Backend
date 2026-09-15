/**
##
## OverDrive 2026
## All Technical rights reserved
##
## catalog_controller_test.go - Package httpadapter source file for services/championship-service/src/adapters/http.
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

	"overdrive/services/championship-service/src/core/domain"
)

// fakeCatalogUseCase implements ports.CatalogQueryUseCase for controller-level tests.
type fakeCatalogUseCase struct {
	championships    []domain.ChampionshipSummary
	championshipsErr error

	events  []domain.EventSummary
	sessErr error

	event    *domain.EventSummary
	eventErr error

	sessions   []domain.SessionSummary
	sessionErr error

	session          *domain.SessionSummary
	getSessionErr    error
	lastGetSessionID string

	drivers         []domain.DriverSummary
	driversErr      error
	lastDriversTeam string
	lastDriversSess string

	teams    []domain.TeamSummary
	teamsErr error

	dataset    domain.SessionDatasetResponse
	datasetErr error

	standings         []domain.StandingRow
	standingsErr      error
	lastStandingsSess string
	lastDriverNumber  *int

	driverProfile     *domain.DriverProfile
	driverProfileErr  error
	lastProfileNumber int
	lastProfileCode   string
}

func (f *fakeCatalogUseCase) ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error) {
	return f.championships, f.championshipsErr
}

func (f *fakeCatalogUseCase) ListEventsByChampionship(ctx context.Context, code string) ([]domain.EventSummary, error) {
	return f.events, f.sessErr
}

func (f *fakeCatalogUseCase) GetEvent(ctx context.Context, eventID string) (*domain.EventSummary, error) {
	return f.event, f.eventErr
}

func (f *fakeCatalogUseCase) ListSessionsByEvent(ctx context.Context, eventID string) ([]domain.SessionSummary, error) {
	return f.sessions, f.sessionErr
}

func (f *fakeCatalogUseCase) GetSession(ctx context.Context, sessionID string) (*domain.SessionSummary, error) {
	f.lastGetSessionID = sessionID
	return f.session, f.getSessionErr
}

func (f *fakeCatalogUseCase) ListSessionDrivers(ctx context.Context, sessionID string, teamID string) ([]domain.DriverSummary, error) {
	f.lastDriversSess = sessionID
	f.lastDriversTeam = teamID
	return f.drivers, f.driversErr
}

func (f *fakeCatalogUseCase) ListSessionTeams(ctx context.Context, sessionID string) ([]domain.TeamSummary, error) {
	return f.teams, f.teamsErr
}

func (f *fakeCatalogUseCase) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetResponse, error) {
	return f.dataset, f.datasetErr
}

func (f *fakeCatalogUseCase) GetSessionStandings(ctx context.Context, sessionID string, driverNumber *int) ([]domain.StandingRow, error) {
	f.lastStandingsSess = sessionID
	f.lastDriverNumber = driverNumber
	return f.standings, f.standingsErr
}

func (f *fakeCatalogUseCase) GetDriverProfile(ctx context.Context, driverNumber int, championshipCode string) (*domain.DriverProfile, error) {
	f.lastProfileNumber = driverNumber
	f.lastProfileCode = championshipCode
	return f.driverProfile, f.driverProfileErr
}

// apiErrorEnvelope mirrors shared/apierror's client-facing JSON shape.
type apiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

// TestCatalogController_GetSessionDataset_ErrorEnvelope is this service's Criterion 6
// representative endpoint: exercises the shared 404/400/500 error contract through the real
// apierror envelope, and proves no raw technical error text leaks into the response body.
func TestCatalogController_GetSessionDataset_ErrorEnvelope(t *testing.T) {
	technicalDetail := "pq: syntax error near SELECT, conn=9f31"

	tests := []struct {
		name           string
		usecase        *fakeCatalogUseCase
		wantStatus     int
		wantCode       string
		mustNotContain string
	}{
		{
			name:       "unknown session -> 404",
			usecase:    &fakeCatalogUseCase{dataset: domain.SessionDatasetResponse{}},
			wantStatus: http.StatusNotFound,
			wantCode:   "SESSION_NOT_FOUND",
		},
		{
			name:       "unknown dataset name -> 400",
			usecase:    &fakeCatalogUseCase{datasetErr: domain.ErrUnknownDataset},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:           "repository failure -> 500, no leaked technical detail",
			usecase:        &fakeCatalogUseCase{datasetErr: errors.New(technicalDetail)},
			wantStatus:     http.StatusInternalServerError,
			wantCode:       "INTERNAL_ERROR",
			mustNotContain: technicalDetail,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := NewCatalogController(tc.usecase)

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
			if envelope.Error.Message == "" {
				t.Fatal("error.message must not be empty")
			}
			if tc.mustNotContain != "" && strings.Contains(rec.Body.String(), tc.mustNotContain) {
				t.Fatalf("response body leaked technical error detail: %s", rec.Body.String())
			}
		})
	}
}

// TestCatalogController_ListChampionshipEvents_SeasonFilter proves the ?season= query filter -
// applied in the controller itself, not the usecase (see catalog.controller.go) - narrows the
// usecase's full result set to matching SeasonYear rows, and that an invalid season value is a
// 400 validation error rather than silently ignored.
func TestCatalogController_ListChampionshipEvents_SeasonFilter(t *testing.T) {
	usecase := &fakeCatalogUseCase{events: []domain.EventSummary{
		{ID: "e1", SeasonYear: 2025},
		{ID: "e2", SeasonYear: 2026},
		{ID: "e3", SeasonYear: 2026},
	}}
	controller := NewCatalogController(usecase)

	t.Run("filters to the requested season", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/championships/f1-2026/events?season=2026", nil)
		req.SetPathValue("code", "f1-2026")
		rec := httptest.NewRecorder()

		controller.ListChampionshipEvents(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var got []domain.EventSummary
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode bare array response: %v (body: %s)", err, rec.Body.String())
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 events for season 2026, got %d: %+v", len(got), got)
		}
	})

	t.Run("invalid season value is a 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/championships/f1-2026/events?season=not-a-number", nil)
		req.SetPathValue("code", "f1-2026")
		rec := httptest.NewRecorder()

		controller.ListChampionshipEvents(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_ListEventSessions_TypeFilter proves the ?type= query filter narrows to
// matching session types case-insensitively, and that "qualifying" is normalized to the
// storage-level "quali" type before filtering (see catalog.controller.go).
func TestCatalogController_ListEventSessions_TypeFilter(t *testing.T) {
	usecase := &fakeCatalogUseCase{sessions: []domain.SessionSummary{
		{ID: "s1", Type: "quali"},
		{ID: "s2", Type: "race"},
	}}
	controller := NewCatalogController(usecase)

	req := httptest.NewRequest(http.MethodGet, "/events/e1/sessions?type=Qualifying", nil)
	req.SetPathValue("eventId", "e1")
	rec := httptest.NewRecorder()

	controller.ListEventSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got []domain.SessionSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode bare array response: %v (body: %s)", err, rec.Body.String())
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Fatalf("expected only the quali session to match ?type=Qualifying, got %+v", got)
	}
}
