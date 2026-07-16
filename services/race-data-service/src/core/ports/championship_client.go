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

// ChampionshipDriverRef, ChampionshipSessionRef, and ChampionshipEventRef mirror
// championship-service's camelCase public contract (see .story/endpoint.md).
type ChampionshipDriverRef struct {
	DriverNumber int    `json:"driverNumber"`
	DriverName   string `json:"fullName"`
	TeamID       string `json:"teamId"`
	TeamName     string `json:"teamName"`
	TeamColor    string `json:"teamColor"`
}

type ChampionshipSessionRef struct {
	ID           string `json:"sessionId"`
	EventID      string `json:"eventId"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Name         string `json:"name"`
	ExternalKey  string `json:"externalKey"`
	BroadcastURL string `json:"broadcastUrl"`
	StartedAtUTC string `json:"startTime"`
	EndedAtUTC   string `json:"endTime"`
}

type ChampionshipEventRef struct {
	ID          string `json:"eventId"`
	Name        string `json:"name"`
	CountryName string `json:"countryName"`
	SeasonYear  int    `json:"seasonYear"`
	ExternalKey string `json:"externalKey"`
}

// ChampionshipClient proxies read calls to championship-service. Note that
// championship-service's public contract returns bare JSON arrays/objects
// (camelCase, no {"count","data"} envelope) as of 2026-07-08 — the
// pass-through methods below therefore return `any` and forward whatever
// shape championship-service produces, instead of assuming an envelope.
type ChampionshipClient interface {
	ListChampionships(ctx context.Context) (any, error)
	ListChampionshipEvents(ctx context.Context, code string) (any, error)
	GetEventPayload(ctx context.Context, eventID string) (any, error)
	ListEventSessions(ctx context.Context, eventID string) (any, error)
	GetSession(ctx context.Context, sessionID string) (*ChampionshipSessionRef, error)
	GetEvent(ctx context.Context, eventID string) (*ChampionshipEventRef, error)
	ListSessionDrivers(ctx context.Context, sessionID string) ([]ChampionshipDriverRef, error)
	ListSessionTeams(ctx context.Context, sessionID string) ([]map[string]any, error)
	GetSessionDataset(ctx context.Context, sessionID string, dataset string) (map[string]any, error)
	GetSessionRaceStandings(ctx context.Context, sessionID string) (any, error)
	GetSessionBroadcast(ctx context.Context, sessionID string) (map[string]any, error)
}
