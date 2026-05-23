/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## catalog.entity.go - Package domain source file for services/championship-service/src/core/domain.
	##
*/

package domain

import "time"

type ChampionshipSummary struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	IsActive bool   `json:"is_active"`
}

type EventSummary struct {
	ID             string    `json:"id"`
	ChampionshipID string    `json:"championship_id"`
	SeasonYear     int       `json:"season_year"`
	RoundNumber    *int      `json:"round_number,omitempty"`
	Name           string    `json:"name"`
	OfficialName   string    `json:"official_name,omitempty"`
	Location       string    `json:"location"`
	CountryName    string    `json:"country_name,omitempty"`
	CountryCode    string    `json:"country_code,omitempty"`
	CircuitName    string    `json:"circuit_name,omitempty"`
	ExternalKey    string    `json:"external_key,omitempty"`
	Status         string    `json:"status"`
	StartsAtUTC    time.Time `json:"starts_at_utc"`
	EndsAtUTC      time.Time `json:"ends_at_utc"`
}

type SessionSummary struct {
	ID           string     `json:"id"`
	EventID      string     `json:"event_id"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	Name         string     `json:"name,omitempty"`
	ExternalKey  string     `json:"external_key,omitempty"`
	BroadcastURL string     `json:"broadcast_url,omitempty"`
	StartedAtUTC time.Time  `json:"started_at_utc"`
	EndedAtUTC   *time.Time `json:"ended_at_utc,omitempty"`
}

type DriverSummary struct {
	ID           string `json:"id"`
	DriverNumber int    `json:"driver_number"`
	DriverName   string `json:"driver_name"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Code         string `json:"code,omitempty"`
	CountryCode  string `json:"country_code,omitempty"`
	TeamName     string `json:"team_name,omitempty"`
	TeamColor    string `json:"team_color,omitempty"`
}

type TeamSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code,omitempty"`
	ColorHex string `json:"color_hex,omitempty"`
}

type SessionDatasetResponse struct {
	SessionID string           `json:"session_id"`
	Dataset   string           `json:"dataset"`
	Count     int              `json:"count"`
	Data      []map[string]any `json:"data"`
	Metadata  map[string]any   `json:"metadata"`
}
