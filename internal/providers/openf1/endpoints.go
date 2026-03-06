/**
##
## OverDrive 2026
## All Technical rights reserved
##
## endpoints.go - Provider endpoint lists for full-race and driver-focused collection modes.
##
*/

package openf1

// RaceSessionEndpoints lists datasets fetched for the full race session.
// Some endpoints may return [] depending on weekend data availability.
var RaceSessionEndpoints = []string{
	"drivers",
	"laps",
	"car_data",
	"location",
	"position",
	"intervals",
	"stints",
	"pit",
	"race_control",
	"team_radio",
	"weather",
	"session_result",
	"starting_grid",
	"overtakes",
	"championship_drivers",
	"championship_teams",
}

// DriverFocusedEndpoints keeps only datasets needed for a single driver view.
var DriverFocusedEndpoints = []string{
	"drivers",
	"laps",
	"car_data",
	"location",
	"position",
	"intervals",
	"stints",
	"pit",
	"team_radio",
	"session_result",
	"starting_grid",
	"overtakes",
	"championship_drivers",
}
