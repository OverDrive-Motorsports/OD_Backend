/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed.entity.go - Per-driver onboard camera feed list exposed under /drivers/{n}/broadcast.
##
*/

package domain

// DriverBroadcast is the payload of GET/PUT /sessions/{sessionId}/drivers/{driverNumber}/broadcast:
// the driver's onboard camera feeds for that session. Feeds is always a JSON array, never null.
type DriverBroadcast struct {
	SessionID    string `json:"sessionId"`
	DriverNumber int    `json:"driverNumber"`
	Feeds        []Feed `json:"feeds"`
}
