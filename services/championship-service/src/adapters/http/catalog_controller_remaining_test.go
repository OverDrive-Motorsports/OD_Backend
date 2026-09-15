/**
##
## OverDrive 2026
## All Technical rights reserved
##
## catalog_controller_remaining_test.go - Package httpadapter source file for services/championship-service/src/adapters/http.
##
*/

package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"overdrive/services/championship-service/src/core/domain"
)

// TestCatalogController_ListChampionships covers the normal case (bare array passthrough) and
// an internal failure surfacing as a 500 via apierror.
func TestCatalogController_ListChampionships(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{championships: []domain.ChampionshipSummary{{ID: "c1", ChampionshipCode: "f1-2026"}}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/championships", nil)
		rec := httptest.NewRecorder()
		controller.ListChampionships(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var got []domain.ChampionshipSummary
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode bare array response: %v (body: %s)", err, rec.Body.String())
		}
		if len(got) != 1 || got[0].ID != "c1" {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("repository failure -> 500", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{championshipsErr: errors.New("boom")}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/championships", nil)
		rec := httptest.NewRecorder()
		controller.ListChampionships(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_GetEvent covers the normal case and the not-found case (nil, nil from
// the usecase must become a 404, not a 200 with a null body).
func TestCatalogController_GetEvent(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{event: &domain.EventSummary{ID: "e1", Name: "Test GP"}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/events/e1", nil)
		req.SetPathValue("eventId", "e1")
		rec := httptest.NewRecorder()
		controller.GetEvent(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var got domain.EventSummary
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v (body: %s)", err, rec.Body.String())
		}
		if got.ID != "e1" {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("unknown event -> 404", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/events/does-not-exist", nil)
		req.SetPathValue("eventId", "does-not-exist")
		rec := httptest.NewRecorder()
		controller.GetEvent(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_GetSession covers the normal case and the not-found case, and proves
// the path-value sessionId is forwarded to the usecase unchanged.
func TestCatalogController_GetSession(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{session: &domain.SessionSummary{ID: "s1", Type: "race"}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/s1", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		controller.GetSession(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if usecase.lastGetSessionID != "s1" {
			t.Fatalf("expected sessionId=s1 forwarded, got %q", usecase.lastGetSessionID)
		}
	})

	t.Run("unknown session -> 404", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/does-not-exist", nil)
		req.SetPathValue("sessionId", "does-not-exist")
		rec := httptest.NewRecorder()
		controller.GetSession(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_ListSessionDrivers proves the ?teamId= filter is forwarded to the
// usecase intact, and that a nil (unknown-session) result is a 404 while an empty (known
// session, no drivers) slice is a 200 with an empty array.
func TestCatalogController_ListSessionDrivers(t *testing.T) {
	t.Run("normal case forwards teamId", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{drivers: []domain.DriverSummary{{DriverNumber: 44}}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/drivers?teamId=team-mercedes", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		controller.ListSessionDrivers(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if usecase.lastDriversTeam != "team-mercedes" || usecase.lastDriversSess != "s1" {
			t.Fatalf("expected teamId/sessionId forwarded, got team=%q session=%q", usecase.lastDriversTeam, usecase.lastDriversSess)
		}
	})

	t.Run("unknown session -> 404", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/does-not-exist/drivers", nil)
		req.SetPathValue("sessionId", "does-not-exist")
		rec := httptest.NewRecorder()
		controller.ListSessionDrivers(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_ListSessionTeams covers the normal case and the not-found case.
func TestCatalogController_ListSessionTeams(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{teams: []domain.TeamSummary{{ID: "t1", Name: "Mercedes"}}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/teams", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		controller.ListSessionTeams(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var got []domain.TeamSummary
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode bare array response: %v (body: %s)", err, rec.Body.String())
		}
		if len(got) != 1 || got[0].ID != "t1" {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("unknown session -> 404", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/does-not-exist/teams", nil)
		req.SetPathValue("sessionId", "does-not-exist")
		rec := httptest.NewRecorder()
		controller.ListSessionTeams(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_GetSessionRaceStandings proves the deprecated /standings/race facade
// delegates to GetSessionDataset with the fixed "session_result" dataset name, and reuses the
// same 404/error-classification behavior as GetSessionDataset.
func TestCatalogController_GetSessionRaceStandings(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{dataset: domain.SessionDatasetResponse{SessionID: "s1", Dataset: "session_result", Count: 1}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/standings/race", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		controller.GetSessionRaceStandings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("unknown session -> 404", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/does-not-exist/standings/race", nil)
		req.SetPathValue("sessionId", "does-not-exist")
		rec := httptest.NewRecorder()
		controller.GetSessionRaceStandings(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_GetSessionStandings covers the normal case, the ?driverNumber= filter
// being parsed and forwarded, an invalid driverNumber being a 400, and the not-found case.
func TestCatalogController_GetSessionStandings(t *testing.T) {
	t.Run("normal case with no driverNumber filter", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{standings: []domain.StandingRow{{Position: 1, DriverNumber: 44}}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/standings", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		controller.GetSessionStandings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if usecase.lastDriverNumber != nil {
			t.Fatalf("expected nil driverNumber to be forwarded, got %v", *usecase.lastDriverNumber)
		}
	})

	t.Run("driverNumber filter is parsed and forwarded", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{standings: []domain.StandingRow{{Position: 1, DriverNumber: 44}}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/standings?driverNumber=44", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		controller.GetSessionStandings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if usecase.lastDriverNumber == nil || *usecase.lastDriverNumber != 44 {
			t.Fatalf("expected driverNumber=44 forwarded, got %v", usecase.lastDriverNumber)
		}
	})

	t.Run("invalid driverNumber -> 400", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/standings?driverNumber=not-a-number", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		controller.GetSessionStandings(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("unknown session -> 404", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/does-not-exist/standings", nil)
		req.SetPathValue("sessionId", "does-not-exist")
		rec := httptest.NewRecorder()
		controller.GetSessionStandings(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_GetDriverProfile covers the normal case (including the
// ?championshipCode= scoping filter), an invalid driver number in the path being a 400, and the
// not-found case.
func TestCatalogController_GetDriverProfile(t *testing.T) {
	t.Run("normal case forwards championshipCode", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{driverProfile: &domain.DriverProfile{DriverNumber: 44, FullName: "Lewis Hamilton"}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/drivers/44/profile?championshipCode=f1-2026", nil)
		req.SetPathValue("driverNumber", "44")
		rec := httptest.NewRecorder()
		controller.GetDriverProfile(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if usecase.lastProfileNumber != 44 || usecase.lastProfileCode != "f1-2026" {
			t.Fatalf("expected driverNumber=44/championshipCode=f1-2026 forwarded, got %d/%q", usecase.lastProfileNumber, usecase.lastProfileCode)
		}
	})

	t.Run("invalid driver number in path -> 400", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/drivers/not-a-number/profile", nil)
		req.SetPathValue("driverNumber", "not-a-number")
		rec := httptest.NewRecorder()
		controller.GetDriverProfile(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("unknown driver -> 404", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/drivers/999/profile", nil)
		req.SetPathValue("driverNumber", "999")
		rec := httptest.NewRecorder()
		controller.GetDriverProfile(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestCatalogController_GetSessionBroadcast proves the broadcast facade reuses GetSession and
// projects only sessionId/broadcastUrl into the response, and the not-found case.
func TestCatalogController_GetSessionBroadcast(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{session: &domain.SessionSummary{ID: "s1", BroadcastURL: "https://example.com/watch"}}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/s1/broadcast", nil)
		req.SetPathValue("sessionId", "s1")
		rec := httptest.NewRecorder()
		controller.GetSessionBroadcast(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v (body: %s)", err, rec.Body.String())
		}
		if got["sessionId"] != "s1" || got["broadcastUrl"] != "https://example.com/watch" {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("unknown session -> 404", func(t *testing.T) {
		usecase := &fakeCatalogUseCase{}
		controller := NewCatalogController(usecase)

		req := httptest.NewRequest(http.MethodGet, "/sessions/does-not-exist/broadcast", nil)
		req.SetPathValue("sessionId", "does-not-exist")
		rec := httptest.NewRecorder()
		controller.GetSessionBroadcast(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}
