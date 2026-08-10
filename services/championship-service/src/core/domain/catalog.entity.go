/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## catalog.entity.go - Package domain source file for services/championship-service/src/core/domain.
	##
*/

package domain

import "time"

// NOTE: all response field names are camelCase — this is the public contract
// consumed by the gateway's /v1/championship/* routes (AR/mobile clients).

type ChampionshipSummary struct {
	ID               string `json:"id"`
	ChampionshipCode string `json:"championshipCode"`
	Name             string `json:"name"`
	Provider         string `json:"provider,omitempty"`
	Category         string `json:"category,omitempty"`
	Season           int    `json:"season,omitempty"`
	IsActive         bool   `json:"isActive"`
}

type EventSummary struct {
	ID               string    `json:"eventId"`
	ChampionshipID   string    `json:"championshipId,omitempty"`
	ChampionshipCode string    `json:"championshipCode,omitempty"`
	SeasonYear       int       `json:"seasonYear,omitempty"`
	RoundNumber      *int      `json:"roundNumber,omitempty"`
	Name             string    `json:"name"`
	OfficialName     string    `json:"officialName,omitempty"`
	Circuit          string    `json:"circuit,omitempty"`
	Location         string    `json:"location"`
	CountryName      string    `json:"countryName,omitempty"`
	CountryCode      string    `json:"countryCode,omitempty"`
	ExternalKey      string    `json:"externalKey,omitempty"`
	Status           string    `json:"status"`
	StartDate        time.Time `json:"startDate"`
	EndDate          time.Time `json:"endDate"`
}

type SessionSummary struct {
	ID           string     `json:"sessionId"`
	EventID      string     `json:"eventId"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	Name         string     `json:"name,omitempty"`
	Circuit      string     `json:"circuit,omitempty"`
	ExternalKey  string     `json:"externalKey,omitempty"`
	BroadcastURL string     `json:"broadcastUrl,omitempty"`
	StartTime    time.Time  `json:"startTime"`
	EndTime      *time.Time `json:"endTime,omitempty"`
}

type DriverSummary struct {
	ID           string `json:"id,omitempty"`
	DriverNumber int    `json:"driverNumber"`
	FullName     string `json:"fullName"`
	FirstName    string `json:"firstName,omitempty"`
	LastName     string `json:"lastName,omitempty"`
	Code         string `json:"code,omitempty"`
	CountryCode  string `json:"countryCode,omitempty"`
	TeamID       string `json:"teamId,omitempty"`
	TeamName     string `json:"teamName,omitempty"`
	TeamColor    string `json:"teamColor,omitempty"`
}

type TeamSummary struct {
	ID       string `json:"teamId"`
	Name     string `json:"name"`
	Code     string `json:"code,omitempty"`
	ColorHex string `json:"color,omitempty"`
}

// StandingRow represents a single row of a session's generic standings
// (race result, starting grid, or championship standings), generalized
// under GET /sessions/{sessionId}/standings.
type StandingRow struct {
	Position     int     `json:"position"`
	DriverNumber int     `json:"driverNumber"`
	TeamID       string  `json:"teamId,omitempty"`
	GapToLeader  string  `json:"gapToLeader,omitempty"`
	Points       float64 `json:"points,omitempty"`
}

// DriverProfile is the global (session-independent) driver profile,
// exposed under GET /drivers/{driverNumber}/profile.
type DriverProfile struct {
	DriverNumber     int    `json:"driverNumber"`
	FullName         string `json:"fullName"`
	Nationality      string `json:"nationality,omitempty"`
	CurrentTeamID    string `json:"currentTeamId,omitempty"`
	ChampionshipCode string `json:"championshipCode,omitempty"`
	DriverPicture    string `json:"driverPicture,omitempty"`
}

type SessionDatasetResponse struct {
	SessionID string           `json:"sessionId"`
	Dataset   string           `json:"dataset"`
	Count     int              `json:"count"`
	Data      []map[string]any `json:"data"`
	Metadata  map[string]any   `json:"metadata"`
}
