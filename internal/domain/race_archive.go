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
