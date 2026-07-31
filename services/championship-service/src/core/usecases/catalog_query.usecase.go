/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## catalog_query.usecase.go - Package usecases source file for services/championship-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"

	"overdrive/services/championship-service/src/core/domain"
	"overdrive/services/championship-service/src/core/ports"
)

type CatalogQueryUseCase struct {
	repository ports.CatalogRepository
}

// NewCatalogQueryUseCase builds and returns a catalog query use case with its required dependencies.
func NewCatalogQueryUseCase(repository ports.CatalogRepository) *CatalogQueryUseCase {
	return &CatalogQueryUseCase{repository: repository}
}

// ListChampionships returns a collection of championships for the requested context.
func (u *CatalogQueryUseCase) ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error) {
	return u.repository.ListChampionships(ctx)
}

// ListEventsByChampionship returns a collection of events by championship for the requested context.
func (u *CatalogQueryUseCase) ListEventsByChampionship(ctx context.Context, code string) ([]domain.EventSummary, error) {
	return u.repository.ListEventsByChampionship(ctx, code)
}

// GetEvent returns the requested event payload for the supplied identifiers.
func (u *CatalogQueryUseCase) GetEvent(ctx context.Context, eventID string) (*domain.EventSummary, error) {
	return u.repository.GetEvent(ctx, eventID)
}

// ListSessionsByEvent returns a collection of sessions by event for the requested context.
func (u *CatalogQueryUseCase) ListSessionsByEvent(ctx context.Context, eventID string) ([]domain.SessionSummary, error) {
	return u.repository.ListSessionsByEvent(ctx, eventID)
}

// GetSession returns the requested session payload for the supplied identifiers.
func (u *CatalogQueryUseCase) GetSession(ctx context.Context, sessionID string) (*domain.SessionSummary, error) {
	return u.repository.GetSession(ctx, sessionID)
}

// ListSessionDrivers returns a collection of session drivers for the requested context.
func (u *CatalogQueryUseCase) ListSessionDrivers(ctx context.Context, sessionID string, teamID string) ([]domain.DriverSummary, error) {
	return u.repository.ListSessionDrivers(ctx, sessionID, teamID)
}

// ListSessionTeams returns a collection of session teams for the requested context.
func (u *CatalogQueryUseCase) ListSessionTeams(ctx context.Context, sessionID string) ([]domain.TeamSummary, error) {
	return u.repository.ListSessionTeams(ctx, sessionID)
}

// GetSessionDataset returns the requested session dataset payload for the supplied identifiers.
func (u *CatalogQueryUseCase) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetResponse, error) {
	return u.repository.GetSessionDataset(ctx, sessionID, dataset)
}

// GetSessionStandings returns the generic session standings, optionally isolating a driver.
func (u *CatalogQueryUseCase) GetSessionStandings(ctx context.Context, sessionID string, driverNumber *int) ([]domain.StandingRow, error) {
	return u.repository.GetSessionStandings(ctx, sessionID, driverNumber)
}

// GetDriverProfile returns the global driver profile for the requested driver number.
func (u *CatalogQueryUseCase) GetDriverProfile(ctx context.Context, driverNumber int, championshipCode string) (*domain.DriverProfile, error) {
	return u.repository.GetDriverProfile(ctx, driverNumber, championshipCode)
}
