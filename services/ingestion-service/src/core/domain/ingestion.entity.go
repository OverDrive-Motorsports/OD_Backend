/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.entity.go - Package domain source file for services/ingestion-service/src/core/domain.
	##
*/

package domain

import "time"

type OpenF1IngestionRequest struct {
	MeetingKey   int      `json:"meeting_key"`
	SessionKey   int      `json:"session_key"`
	DriverNumber *int     `json:"driver_number,omitempty"`
	Resources    []string `json:"resources,omitempty"`
	Dispatch     *bool    `json:"dispatch,omitempty"`
}

type ResourceIngestion struct {
	Resource      string `json:"resource"`
	DatasetName   string `json:"dataset_name"`
	TargetService string `json:"target_service"`
	RowCount      int    `json:"row_count"`
	Dispatched    bool   `json:"dispatched"`
}

type ServiceDispatch struct {
	Service      string `json:"service"`
	DatasetCount int    `json:"dataset_count"`
	RowCount     int    `json:"row_count"`
}

type OpenF1IngestionResult struct {
	Provider        string              `json:"provider"`
	BatchID         string              `json:"batch_id"`
	MeetingKey      int                 `json:"meeting_key"`
	SessionKey      int                 `json:"session_key"`
	DriverNumber    *int                `json:"driver_number,omitempty"`
	Resources       []ResourceIngestion `json:"resources"`
	Dispatches      []ServiceDispatch   `json:"dispatches"`
	DispatchEnabled bool                `json:"dispatch_enabled"`
	TriggeredAtUTC  time.Time           `json:"triggered_at_utc"`
	CompletedAtUTC  time.Time           `json:"completed_at_utc"`
}
