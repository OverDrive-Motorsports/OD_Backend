/**
##
## OverDrive 2026
## All Technical rights reserved
##
## batch.go - Package ingestion source file for shared/contracts/ingestion.
##
*/

package ingestion

import "time"

type Context struct {
	MeetingKey   int    `json:"meeting_key,omitempty"`
	SessionKey   int    `json:"session_key,omitempty"`
	DriverNumber *int   `json:"driver_number,omitempty"`
	Provider     string `json:"provider"`
}

type Dataset struct {
	Name             string           `json:"name"`
	Category         string           `json:"category"`
	TargetService    string           `json:"target_service"`
	ProviderResource string           `json:"provider_resource"`
	SchemaVersion    string           `json:"schema_version"`
	RowCount         int              `json:"row_count"`
	Rows             []map[string]any `json:"rows"`
	Metadata         map[string]any   `json:"metadata,omitempty"`
}

type Batch struct {
	BatchID     string         `json:"batch_id"`
	Provider    string         `json:"provider"`
	TriggeredAt time.Time      `json:"triggered_at"`
	Context     Context        `json:"context"`
	Datasets    []Dataset      `json:"datasets"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type DispatchAck struct {
	Service        string    `json:"service"`
	BatchID        string    `json:"batch_id"`
	Accepted       bool      `json:"accepted"`
	DatasetCount   int       `json:"dataset_count"`
	TotalRowCount  int       `json:"total_row_count"`
	ProcessedAtUtc time.Time `json:"processed_at_utc"`
}
