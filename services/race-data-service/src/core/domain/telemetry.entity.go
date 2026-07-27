/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## telemetry.entity.go - Package domain source file for services/race-data-service/src/core/domain.
	##
*/

package domain

import "time"

// TelemetrySpeed is a single instantaneous speed sample (changed 2026-07-08
// from an aggregate current/top/average summary to the full raw sample list,
// mirroring TelemetryLocation's shape — GET /telemetry/speed now returns
// every sample for the session/driver, not one computed snapshot).
// See .story/endpoint.md.
type TelemetrySpeed struct {
	Speed     int       `json:"speed"`
	Gear      int       `json:"gear"`
	Timestamp time.Time `json:"timestamp"`
}

// TelemetryEngine is a single instantaneous engine sample (same 2026-07-08
// change as TelemetrySpeed: full raw sample list instead of a "latest" snapshot).
// NOTE: `battery` (mode/percentage) is intentionally dropped from this contract —
// no data source is currently available (validated 2026-07-08).
type TelemetryEngine struct {
	Rpm             int       `json:"rpm"`
	Gear            int       `json:"gear"`
	ThrottlePercent float64   `json:"throttlePercent"`
	BrakePercent    float64   `json:"brakePercent"`
	DrsActive       bool      `json:"drsActive"`
	Timestamp       time.Time `json:"timestamp"`
}

// TelemetryLocation is a single spatial sample (x, y, z).
type TelemetryLocation struct {
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Z         float64   `json:"z"`
	Timestamp time.Time `json:"timestamp"`
}

// TelemetryIntervals reports real-time gaps to the leader / driver ahead.
type TelemetryIntervals struct {
	DriverNumber int       `json:"driverNumber"`
	GapToLeader  string    `json:"gapToLeader,omitempty"`
	GapAhead     *string   `json:"gapAhead"`
	Timestamp    time.Time `json:"timestamp"`
}
