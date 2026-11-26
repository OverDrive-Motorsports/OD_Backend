/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_query.service.go - Package ports source file for services/race-data-service/src/core/ports.
##
*/

package ports

import (
	"context"

	"overdrive/services/race-data-service/src/core/domain"
)

type SessionQueryUseCase interface {
	ListChampionships(ctx context.Context) (any, error)
	ListChampionshipEvents(ctx context.Context, code string) (any, error)
	GetEvent(ctx context.Context, eventID string) (any, error)
	ListEventSessions(ctx context.Context, eventID string) (any, error)
	GetSession(ctx context.Context, sessionID string) (map[string]any, error)
	GetSessionMetadata(ctx context.Context, sessionID string) (map[string]any, error)
	ListSessionDrivers(ctx context.Context, sessionID string) (map[string]any, error)
	ListSessionTeams(ctx context.Context, sessionID string) (map[string]any, error)
	// GetSessionDataset and GetDriverDataset return one page and the next page offset (nil on
	// the last page, and always nil for the datasets served by championship-service).
	GetSessionDataset(ctx context.Context, sessionID string, dataset string, page domain.Page) (map[string]any, *int, error)
	GetSessionRaceStandings(ctx context.Context, sessionID string) (any, error)
	GetDriverProfile(ctx context.Context, sessionID string, driverNumber int) (map[string]any, error)
	GetDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string, page domain.Page) (map[string]any, *int, error)
	GetDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (map[string]any, error)
	GetSessionFacts(ctx context.Context, sessionID string) (map[string]any, error)
}
