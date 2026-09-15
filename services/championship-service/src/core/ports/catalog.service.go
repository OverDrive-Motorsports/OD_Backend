/**
##
## OverDrive 2026
## All Technical rights reserved
##
## catalog.service.go - Package ports source file for services/championship-service/src/core/ports.
##
*/

package ports

import (
	"context"

	"overdrive/services/championship-service/src/core/domain"
)

type CatalogQueryUseCase interface {
	ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error)
	ListEventsByChampionship(ctx context.Context, code string) ([]domain.EventSummary, error)
	GetEvent(ctx context.Context, eventID string) (*domain.EventSummary, error)
	ListSessionsByEvent(ctx context.Context, eventID string) ([]domain.SessionSummary, error)
	GetSession(ctx context.Context, sessionID string) (*domain.SessionSummary, error)
	ListSessionDrivers(ctx context.Context, sessionID string, teamID string) ([]domain.DriverSummary, error)
	ListSessionTeams(ctx context.Context, sessionID string) ([]domain.TeamSummary, error)
	GetSessionDataset(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetResponse, error)
	GetSessionStandings(ctx context.Context, sessionID string, driverNumber *int) ([]domain.StandingRow, error)
	GetDriverProfile(ctx context.Context, driverNumber int, championshipCode string) (*domain.DriverProfile, error)
}
