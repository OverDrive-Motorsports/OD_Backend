/**
##
## OverDrive 2026
## All Technical rights reserved
##
## catalog.go - Domain DTOs for championship, event, and session catalog endpoints.
##
*/

package domain

import "time"

// ChampionshipSummary describes one stored championship entry.
type ChampionshipSummary struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Category  string    `json:"category,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EventSummary describes one stored event entry.
type EventSummary struct {
	ID             string     `json:"id"`
	ChampionshipID string     `json:"championship_id"`
	SeasonYear     int        `json:"season_year"`
	RoundNumber    *int       `json:"round_number,omitempty"`
	Name           string     `json:"name"`
	OfficialName   string     `json:"official_name,omitempty"`
	CountryName    string     `json:"country_name,omitempty"`
	CountryCode    string     `json:"country_code,omitempty"`
	CircuitName    string     `json:"circuit_name,omitempty"`
	ExternalKey    string     `json:"external_key,omitempty"`
	Status         string     `json:"status"`
	StartsAtUTC    time.Time  `json:"starts_at_utc"`
	EndsAtUTC      *time.Time `json:"ends_at_utc,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// SessionSummary describes one stored session entry.
type SessionSummary struct {
	ID           string         `json:"id"`
	EventID      string         `json:"event_id"`
	Type         string         `json:"type"`
	Status       string         `json:"status"`
	Name         string         `json:"name,omitempty"`
	ExternalKey  string         `json:"external_key,omitempty"`
	BroadcastURL string         `json:"broadcast_url,omitempty"`
	StartedAtUTC time.Time      `json:"started_at_utc"`
	EndedAtUTC   *time.Time     `json:"ended_at_utc,omitempty"`
	LatestCounts map[string]int `json:"latest_counts,omitempty"`
	LatestStored *time.Time     `json:"latest_stored,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}
