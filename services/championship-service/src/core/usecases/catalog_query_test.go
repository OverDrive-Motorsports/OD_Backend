/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## catalog_query_test.go - Package usecases source file for services/championship-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"
	"errors"
	"testing"

	"overdrive/services/championship-service/src/core/domain"
)

// fakeCatalogRepository is an in-memory ports.CatalogRepository used to exercise
// CatalogQueryUseCase without a DB. It also records the last arguments it was called with, so
// tests can prove query-filter parameters (teamId/championshipCode) are forwarded intact.
type fakeCatalogRepository struct {
	championships []domain.ChampionshipSummary
	events        []domain.EventSummary
	event         *domain.EventSummary
	sessions      []domain.SessionSummary
	session       *domain.SessionSummary
	drivers       []domain.DriverSummary
	teams         []domain.TeamSummary
	dataset       domain.SessionDatasetResponse
	standings     []domain.StandingRow
	driverProfile *domain.DriverProfile
	err           error

	lastTeamID           string
	lastChampionshipCode string
	lastStandingsDriver  *int
}

func (f *fakeCatalogRepository) ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error) {
	return f.championships, f.err
}

func (f *fakeCatalogRepository) ListEventsByChampionship(ctx context.Context, code string) ([]domain.EventSummary, error) {
	return f.events, f.err
}

func (f *fakeCatalogRepository) GetEvent(ctx context.Context, eventID string) (*domain.EventSummary, error) {
	return f.event, f.err
}

func (f *fakeCatalogRepository) ListSessionsByEvent(ctx context.Context, eventID string) ([]domain.SessionSummary, error) {
	return f.sessions, f.err
}

func (f *fakeCatalogRepository) GetSession(ctx context.Context, sessionID string) (*domain.SessionSummary, error) {
	return f.session, f.err
}

func (f *fakeCatalogRepository) ListSessionDrivers(ctx context.Context, sessionID string, teamID string) ([]domain.DriverSummary, error) {
	f.lastTeamID = teamID
	return f.drivers, f.err
}

func (f *fakeCatalogRepository) ListSessionTeams(ctx context.Context, sessionID string) ([]domain.TeamSummary, error) {
	return f.teams, f.err
}

func (f *fakeCatalogRepository) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetResponse, error) {
	return f.dataset, f.err
}

func (f *fakeCatalogRepository) GetSessionStandings(ctx context.Context, sessionID string, driverNumber *int) ([]domain.StandingRow, error) {
	f.lastStandingsDriver = driverNumber
	return f.standings, f.err
}

func (f *fakeCatalogRepository) GetDriverProfile(ctx context.Context, driverNumber int, championshipCode string) (*domain.DriverProfile, error) {
	f.lastChampionshipCode = championshipCode
	return f.driverProfile, f.err
}

// TestCatalogQueryUseCase_NormalCase proves every method is an unmodified pass-through to the
// repository for the successful/normal case.
func TestCatalogQueryUseCase_NormalCase(t *testing.T) {
	repo := &fakeCatalogRepository{
		championships: []domain.ChampionshipSummary{{ID: "c1", ChampionshipCode: "f1-2026"}},
		events:        []domain.EventSummary{{ID: "e1"}},
		event:         &domain.EventSummary{ID: "e1"},
		sessions:      []domain.SessionSummary{{ID: "s1"}},
		session:       &domain.SessionSummary{ID: "s1"},
		drivers:       []domain.DriverSummary{{DriverNumber: 44}},
		teams:         []domain.TeamSummary{{ID: "t1"}},
		dataset:       domain.SessionDatasetResponse{SessionID: "s1", Dataset: "laps", Count: 1},
		standings:     []domain.StandingRow{{Position: 1, DriverNumber: 1}},
		driverProfile: &domain.DriverProfile{DriverNumber: 44},
	}
	uc := NewCatalogQueryUseCase(repo)
	ctx := context.Background()

	if got, err := uc.ListChampionships(ctx); err != nil || len(got) != 1 {
		t.Fatalf("ListChampionships: got %v, err %v", got, err)
	}
	if got, err := uc.ListEventsByChampionship(ctx, "f1-2026"); err != nil || len(got) != 1 {
		t.Fatalf("ListEventsByChampionship: got %v, err %v", got, err)
	}
	if got, err := uc.GetEvent(ctx, "e1"); err != nil || got == nil || got.ID != "e1" {
		t.Fatalf("GetEvent: got %v, err %v", got, err)
	}
	if got, err := uc.ListSessionsByEvent(ctx, "e1"); err != nil || len(got) != 1 {
		t.Fatalf("ListSessionsByEvent: got %v, err %v", got, err)
	}
	if got, err := uc.GetSession(ctx, "s1"); err != nil || got == nil || got.ID != "s1" {
		t.Fatalf("GetSession: got %v, err %v", got, err)
	}
	if got, err := uc.ListSessionDrivers(ctx, "s1", ""); err != nil || len(got) != 1 {
		t.Fatalf("ListSessionDrivers: got %v, err %v", got, err)
	}
	if got, err := uc.ListSessionTeams(ctx, "s1"); err != nil || len(got) != 1 {
		t.Fatalf("ListSessionTeams: got %v, err %v", got, err)
	}
	if got, err := uc.GetSessionDataset(ctx, "s1", "laps"); err != nil || got.SessionID != "s1" {
		t.Fatalf("GetSessionDataset: got %v, err %v", got, err)
	}
	if got, err := uc.GetSessionStandings(ctx, "s1", nil); err != nil || len(got) != 1 {
		t.Fatalf("GetSessionStandings: got %v, err %v", got, err)
	}
	if got, err := uc.GetDriverProfile(ctx, 44, ""); err != nil || got == nil || got.DriverNumber != 44 {
		t.Fatalf("GetDriverProfile: got %v, err %v", got, err)
	}
}

// TestCatalogQueryUseCase_UnknownResource proves an unknown resource (repository returning a
// nil pointer/nil slice with no error) is forwarded as-is rather than swallowed - the controller
// layer is what turns this into a 404, but the usecase must not mask it into something else.
func TestCatalogQueryUseCase_UnknownResource(t *testing.T) {
	repo := &fakeCatalogRepository{}
	uc := NewCatalogQueryUseCase(repo)
	ctx := context.Background()

	if got, err := uc.GetEvent(ctx, "does-not-exist"); err != nil || got != nil {
		t.Fatalf("GetEvent: expected nil, nil for an unknown event, got %v, %v", got, err)
	}
	if got, err := uc.GetSession(ctx, "does-not-exist"); err != nil || got != nil {
		t.Fatalf("GetSession: expected nil, nil for an unknown session, got %v, %v", got, err)
	}
	if got, err := uc.ListEventsByChampionship(ctx, "does-not-exist"); err != nil || got != nil {
		t.Fatalf("ListEventsByChampionship: expected nil, nil for an unknown championship, got %v, %v", got, err)
	}
	if got, err := uc.GetDriverProfile(ctx, 999, ""); err != nil || got != nil {
		t.Fatalf("GetDriverProfile: expected nil, nil for an unknown driver, got %v, %v", got, err)
	}
}

// TestCatalogQueryUseCase_UnknownDatasetSentinelPropagates proves the domain.ErrUnknownDataset
// sentinel error survives the usecase pass-through unwrapped, since the controller relies on
// errors.Is to classify it as a 400 (see catalog.controller.go's classifyDatasetErr).
func TestCatalogQueryUseCase_UnknownDatasetSentinelPropagates(t *testing.T) {
	repo := &fakeCatalogRepository{err: domain.ErrUnknownDataset}
	uc := NewCatalogQueryUseCase(repo)

	_, err := uc.GetSessionDataset(context.Background(), "s1", "not-a-real-dataset")
	if !errors.Is(err, domain.ErrUnknownDataset) {
		t.Fatalf("expected the sentinel to propagate via errors.Is, got %v", err)
	}
}

// TestCatalogQueryUseCase_FilterParametersForwarded proves the teamId and championshipCode
// query-filter parameters (parsed by the controller from ?teamId=/?championshipCode=) are
// forwarded to the repository intact rather than being reinterpreted by the usecase. The
// season/type filters are applied in the controller itself (post repository-call, in-memory
// slice filtering - see catalog.controller.go), so they are covered at the controller level in
// catalog_controller_test.go instead.
func TestCatalogQueryUseCase_FilterParametersForwarded(t *testing.T) {
	repo := &fakeCatalogRepository{}
	uc := NewCatalogQueryUseCase(repo)
	ctx := context.Background()

	if _, err := uc.ListSessionDrivers(ctx, "s1", "team-red-bull"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastTeamID != "team-red-bull" {
		t.Fatalf("expected teamID=%q to be forwarded, got %q", "team-red-bull", repo.lastTeamID)
	}

	if _, err := uc.GetDriverProfile(ctx, 44, "f1-2026"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastChampionshipCode != "f1-2026" {
		t.Fatalf("expected championshipCode=%q to be forwarded, got %q", "f1-2026", repo.lastChampionshipCode)
	}

	driverNumber := 44
	if _, err := uc.GetSessionStandings(ctx, "s1", &driverNumber); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastStandingsDriver == nil || *repo.lastStandingsDriver != 44 {
		t.Fatalf("expected driverNumber=44 to be forwarded, got %v", repo.lastStandingsDriver)
	}
}
