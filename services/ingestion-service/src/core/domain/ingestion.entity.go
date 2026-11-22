/**
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

// Resource and run statuses of an ingestion result.
const (
	ResourceStatusOK      = "ok"
	ResourceStatusPartial = "partial"
	ResourceStatusFailed  = "failed"

	IngestionStatusCompleted = "completed"
	IngestionStatusPartial   = "partial"
)

// HeavyResources are fetched and dispatched one driver at a time when the request covers the
// whole session: a full session of either is hundreds of megabytes in a single OpenF1 response.
var HeavyResources = map[string]bool{"car_data": true, "location": true}

// ResourceIngestion reports one resource of an ingestion run. Status is ok, partial (a heavy
// resource where some drivers failed, listed in FailedDrivers) or failed, with a short Error.
type ResourceIngestion struct {
	Resource      string `json:"resource"`
	DatasetName   string `json:"dataset_name"`
	TargetService string `json:"target_service"`
	RowCount      int    `json:"row_count"`
	Dispatched    bool   `json:"dispatched"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	FailedDrivers []int  `json:"failed_drivers,omitempty"`
}

type ServiceDispatch struct {
	Service      string `json:"service"`
	DatasetCount int    `json:"dataset_count"`
	RowCount     int    `json:"row_count"`
}

// OpenF1IngestionResult reports an ingestion run. Status is completed when every resource is ok,
// partial otherwise (a run where every resource failed is an error, not a result).
type OpenF1IngestionResult struct {
	Status          string              `json:"status"`
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
