/**
##
## OverDrive 2026
## All Technical rights reserved
##
## catalog_router_test.go - Router tests for championship, race, and session catalog routes.
##
*/

package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"overdrive/internal/api"
	"overdrive/internal/domain"
	"overdrive/internal/service"
	"overdrive/tests/internal/mocks"
)

// TestRouterCatalogRoutes verifies hierarchical catalog routes are wired to their handlers.
func TestRouterCatalogRoutes(t *testing.T) {
	store := &mocks.RaceArchiveStoreMock{
		ListChampionshipsFn: func(ctx context.Context) ([]domain.ChampionshipSummary, error) {
			return []domain.ChampionshipSummary{mocks.SampleChampionship()}, nil
		},
		GetChampionshipRacesFn: func(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.RaceSummary, bool, error) {
			return mocks.SampleChampionship(), []domain.RaceSummary{mocks.SampleRace()}, true, nil
		},
		GetRaceFn: func(ctx context.Context, raceID string) (domain.RaceSummary, bool, error) {
			return mocks.SampleRace(), true, nil
		},
		ListRaceSessionsFn: func(ctx context.Context, raceID string) ([]domain.SessionSummary, bool, error) {
			return []domain.SessionSummary{mocks.SampleSession()}, true, nil
		},
		GetSessionFn: func(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error) {
			return mocks.SampleSession(), true, nil
		},
	}

	router := api.NewRouter(
		api.NewRaceHandler(service.NewRaceService(nil, store), api.RaceDefaults{}, time.Second),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	cases := []struct {
		name           string
		method         string
		target         string
		expectedStatus int
		assert         func(*testing.T, map[string]any)
	}{
		{
			name:           "championship list",
			method:         http.MethodGet,
			target:         "/api/v1/championships",
			expectedStatus: http.StatusOK,
			assert: func(t *testing.T, payload map[string]any) {
				if int(payload["count"].(float64)) != 1 {
					t.Fatalf("unexpected count: %#v", payload["count"])
				}
			},
		},
		{
			name:           "championship races",
			method:         http.MethodGet,
			target:         "/api/v1/championships/f1/races",
			expectedStatus: http.StatusOK,
			assert: func(t *testing.T, payload map[string]any) {
				if int(payload["count"].(float64)) != 1 {
					t.Fatalf("unexpected count: %#v", payload["count"])
				}
			},
		},
		{
			name:           "race by id",
			method:         http.MethodGet,
			target:         "/api/v1/races/race-aus-2025",
			expectedStatus: http.StatusOK,
			assert: func(t *testing.T, payload map[string]any) {
				if payload["id"] != "race-aus-2025" {
					t.Fatalf("unexpected race id: %#v", payload["id"])
				}
			},
		},
		{
			name:           "race sessions",
			method:         http.MethodGet,
			target:         "/api/v1/races/race-aus-2025/sessions",
			expectedStatus: http.StatusOK,
			assert: func(t *testing.T, payload map[string]any) {
				if int(payload["count"].(float64)) != 1 {
					t.Fatalf("unexpected count: %#v", payload["count"])
				}
			},
		},
		{
			name:           "session by id",
			method:         http.MethodGet,
			target:         "/api/v1/sessions/session-race-9693",
			expectedStatus: http.StatusOK,
			assert: func(t *testing.T, payload map[string]any) {
				if payload["id"] != "session-race-9693" {
					t.Fatalf("unexpected session id: %#v", payload["id"])
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to decode payload: %v", err)
			}
			tc.assert(t, payload)
		})
	}
}
