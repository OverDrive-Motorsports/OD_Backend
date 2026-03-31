/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive.go - Domain model describing normalized race archive payload.
##
*/

package domain

import "time"

// RaceArchive is the final payload returned by the API race endpoints.
// Datasets are kept as raw provider rows to preserve full OpenF1 fidelity.
type RaceArchive struct {
	Metadata    Metadata                    `json:"metadata"`
	Meeting     map[string]any              `json:"meeting"`
	AllSessions []map[string]any            `json:"all_sessions"`
	RaceSession map[string]any              `json:"race_session"`
	Datasets    map[string][]map[string]any `json:"datasets"`
	Counts      map[string]int              `json:"counts"`
	FetchErrors map[string]string           `json:"fetch_errors,omitempty"`
}

type Metadata struct {
	GeneratedAt  time.Time `json:"generated_at"`
	Provider     string    `json:"provider"`
	MeetingName  string    `json:"meeting_name"`
	CountryName  string    `json:"country_name"`
	Year         int       `json:"year"`
	MeetingKey   int       `json:"meeting_key"`
	RaceSession  string    `json:"race_session_name"`
	RaceSessKey  int       `json:"race_session_key"`
	EndpointSize int       `json:"endpoint_count"`
	DriverNumber int       `json:"driver_number,omitempty"`
}

// DriverLapLocationWindow represents all stored location samples for one driver during one lap window.
type DriverLapLocationWindow struct {
	Dataset      string           `json:"dataset"`
	SessionID    string           `json:"session_id,omitempty"`
	DriverNumber int              `json:"driver_number"`
	DriverName   string           `json:"driver_name,omitempty"`
	TeamName     string           `json:"team_name,omitempty"`
	LapNumber    int              `json:"lap_number"`
	WindowStart  time.Time        `json:"window_start"`
	WindowEnd    time.Time        `json:"window_end"`
	Count        int              `json:"count"`
	Data         []map[string]any `json:"data"`
	Metadata     map[string]any   `json:"metadata,omitempty"`
}

// DriverDatasetWindow represents one driver-scoped dataset read directly from normalized storage.
type DriverDatasetWindow struct {
	Dataset      string           `json:"dataset"`
	SessionID    string           `json:"session_id,omitempty"`
	DriverNumber int              `json:"driver_number"`
	DriverName   string           `json:"driver_name,omitempty"`
	TeamName     string           `json:"team_name,omitempty"`
	Count        int              `json:"count"`
	Data         []map[string]any `json:"data"`
	Metadata     map[string]any   `json:"metadata,omitempty"`
}

// RaceStandingsWindow represents one race standings snapshot read directly from normalized storage.
type RaceStandingsWindow struct {
	Dataset     string           `json:"dataset"`
	SessionID   string           `json:"session_id,omitempty"`
	SnapshotAt  time.Time        `json:"snapshot_at"`
	RequestedAt *time.Time       `json:"requested_at,omitempty"`
	Count       int              `json:"count"`
	Data        []map[string]any `json:"data"`
	Metadata    map[string]any   `json:"metadata,omitempty"`
}

// SessionDatasetWindow represents one session-scoped dataset read directly from normalized storage.
type SessionDatasetWindow struct {
	Dataset   string           `json:"dataset"`
	SessionID string           `json:"session_id,omitempty"`
	Count     int              `json:"count"`
	Data      []map[string]any `json:"data"`
	Metadata  map[string]any   `json:"metadata,omitempty"`
}

// SessionMetadataWindow represents the light metadata envelope for one session without rebuilding all datasets.
type SessionMetadataWindow struct {
	Metadata    map[string]any   `json:"metadata"`
	Meeting     map[string]any   `json:"meeting"`
	AllSessions []map[string]any `json:"all_sessions"`
	RaceSession map[string]any   `json:"race_session"`
	StoredAt    time.Time        `json:"stored_at"`
}

// SessionDatasetCatalogWindow represents the dataset catalog for one session without decoding all chunks.
type SessionDatasetCatalogWindow struct {
	SessionID string           `json:"session_id,omitempty"`
	Metadata  map[string]any   `json:"metadata"`
	Count     int              `json:"count"`
	Data      []map[string]any `json:"data"`
}
