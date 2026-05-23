/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## championship_client.go - Package ports source file for services/race-data-service/src/core/ports.
	##
*/

package ports

import "context"

type ChampionshipDriverRef struct {
	DriverNumber int
	DriverName   string
	TeamName     string
	TeamColor    string
}

type ChampionshipSessionRef struct {
	ID           string
	EventID      string
	Type         string
	Status       string
	Name         string
	ExternalKey  string
	BroadcastURL string
	StartedAtUTC string
	EndedAtUTC   string
}

type ChampionshipEventRef struct {
	ID          string
	Name        string
	CountryName string
	SeasonYear  int
	ExternalKey string
}

type ChampionshipClient interface {
	ListChampionships(ctx context.Context) (map[string]any, error)
	ListChampionshipEvents(ctx context.Context, code string) (map[string]any, error)
	GetEventPayload(ctx context.Context, eventID string) (map[string]any, error)
	ListEventSessions(ctx context.Context, eventID string) (map[string]any, error)
	GetSession(ctx context.Context, sessionID string) (*ChampionshipSessionRef, error)
	GetEvent(ctx context.Context, eventID string) (*ChampionshipEventRef, error)
	ListSessionDrivers(ctx context.Context, sessionID string) ([]ChampionshipDriverRef, error)
	ListSessionTeams(ctx context.Context, sessionID string) ([]map[string]any, error)
	GetSessionDataset(ctx context.Context, sessionID string, dataset string) (map[string]any, error)
	GetSessionRaceStandings(ctx context.Context, sessionID string) (map[string]any, error)
	GetSessionBroadcast(ctx context.Context, sessionID string) (map[string]any, error)
}
