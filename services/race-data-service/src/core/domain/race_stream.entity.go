/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_stream.entity.go - Package domain source file for services/race-data-service/src/core/domain.
	##
*/

package domain

import "time"

// RaceReplayTelemetryFrame is one raw telemetry sample used to build
// GET /sessions/{sessionId}/race/replay's `telemetry` bulk dump. Unlike
// GET /telemetry/speed, this is a single instantaneous sample: it has no
// topSpeed/averageSpeed aggregate, since those are only meaningful over a
// window, not for one row of a bulk dump. See .story/endpoint.md.
//
// DriverNumber is kept here (unlike the trimmed RaceReplayTelemetrySample
// below) because the repository/usecase need it to group samples by driver
// before it becomes redundant with the response's outer per-driver key.
type RaceReplayTelemetryFrame struct {
	DriverNumber    int       `json:"driverNumber"`
	Speed           int       `json:"speed"`
	Rpm             int       `json:"rpm"`
	Gear            int       `json:"gear"`
	ThrottlePercent float64   `json:"throttlePercent"`
	BrakePercent    float64   `json:"brakePercent"`
	DrsActive       bool      `json:"drsActive"`
	LapNumber       int       `json:"lapNumber"`
	Timestamp       time.Time `json:"timestamp"`
	X               float64   `json:"x"`
	Y               float64   `json:"y"`
	Z               float64   `json:"z"`
}

// RaceReplayTelemetrySample is the `telemetry` bulk dump's per-sample shape:
// RaceReplayTelemetryFrame with driverNumber dropped, since each sample
// already lives under its driver's key in the response's telemetry map.
type RaceReplayTelemetrySample struct {
	Speed           int       `json:"speed"`
	Rpm             int       `json:"rpm"`
	Gear            int       `json:"gear"`
	ThrottlePercent float64   `json:"throttlePercent"`
	BrakePercent    float64   `json:"brakePercent"`
	DrsActive       bool      `json:"drsActive"`
	LapNumber       int       `json:"lapNumber"`
	X               float64   `json:"x"`
	Y               float64   `json:"y"`
	Z               float64   `json:"z"`
	Timestamp       time.Time `json:"timestamp"`
}

// RaceReplayLapEvent mirrors one entry of GET /race/laps' `laps` array (see
// RaceLapEntry) with an added DriverNumber, since the repository/usecase
// group laps for every driver in one call and need it to build the response's
// per-driver `lap` map. There is no timestamp field in the payload itself
// (matching the snapshot row shape exactly) — the moment each lap completed
// is carried alongside it by the repository/usecase, not inside the JSON
// body. See race_stream.repository.go's ListLapFrames for how that moment
// (lap completion, not lap start) is derived.
type RaceReplayLapEvent struct {
	DriverNumber int      `json:"driverNumber"`
	LapNumber    int      `json:"lapNumber"`
	LapDuration  *float64 `json:"lapDuration"`
	Sector1      *float64 `json:"sector1"`
	Sector2      *float64 `json:"sector2"`
	Sector3      *float64 `json:"sector3"`
	IsPitOutLap  bool     `json:"isPitOutLap"`
}

// RaceReplayLapSample is the `lap` bulk dump's per-sample shape:
// RaceReplayLapEvent with driverNumber dropped, since each sample already
// lives under its driver's key in the response's lap map.
type RaceReplayLapSample struct {
	LapNumber   int      `json:"lapNumber"`
	LapDuration *float64 `json:"lapDuration"`
	Sector1     *float64 `json:"sector1"`
	Sector2     *float64 `json:"sector2"`
	Sector3     *float64 `json:"sector3"`
	IsPitOutLap bool     `json:"isPitOutLap"`
}

// RaceReplayPositionSample is the `position` bulk dump's per-sample shape:
// RacePosition with driverNumber dropped. Gap fields are intentionally left
// empty here, same as before — recomputing gaps per historical sample would
// require an extra interval lookup per row, which isn't worth it for a bulk
// dump. Use GET /sessions/{sessionId}/race/position for gap-enriched data.
type RaceReplayPositionSample struct {
	Position      int       `json:"position"`
	GapToLeader   string    `json:"gapToLeader,omitempty"`
	GapAhead      *string   `json:"gapAhead"`
	GapBehind     *string   `json:"gapBehind"`
	LapsCompleted int       `json:"lapsCompleted"`
	Timestamp     time.Time `json:"timestamp"`
}

// RaceReplayRadioSample is the `radio` bulk dump's per-sample shape:
// RaceRadioMessage with driverNumber dropped.
type RaceReplayRadioSample struct {
	Timestamp time.Time `json:"timestamp"`
	AudioURL  string    `json:"audioUrl,omitempty"`
}

// RaceReplayPitStopSample is the `pitStop` bulk dump's per-sample shape:
// RacePitStop with driverNumber dropped.
type RaceReplayPitStopSample struct {
	LapNumber   int     `json:"lapNumber"`
	PitDuration float64 `json:"pitDuration"`
}

// RaceReplayStintSample is the `stint` bulk dump's per-sample shape:
// RaceStint with driverNumber dropped.
type RaceReplayStintSample struct {
	StintNumber    int    `json:"stintNumber"`
	Compound       string `json:"compound,omitempty"`
	LapStart       int    `json:"lapStart"`
	LapEnd         int    `json:"lapEnd"`
	TyreAgeAtStart int    `json:"tyreAgeAtStart"`
}

// RaceReplay is the full response body of
// GET /sessions/{sessionId}/race/replay: a plain JSON bulk dump of every
// stored sample for a session. Driver-scoped datasets are grouped into a map
// keyed by driver number (as a JSON string, e.g. "63"), each value a
// chronologically-sorted (oldest first) array of that driver's samples.
// Track-wide datasets (RaceControl, Weather) stay flat arrays, matching their
// existing snapshot endpoints (GET /race/control, GET /race/weather), which
// never had a driverNumber field to begin with.
type RaceReplay struct {
	Telemetry   map[string][]RaceReplayTelemetrySample `json:"telemetry"`
	Position    map[string][]RaceReplayPositionSample  `json:"position"`
	Radio       map[string][]RaceReplayRadioSample     `json:"radio"`
	PitStop     map[string][]RaceReplayPitStopSample   `json:"pitStop"`
	Stint       map[string][]RaceReplayStintSample     `json:"stint"`
	Lap         map[string][]RaceReplayLapSample       `json:"lap"`
	RaceControl []RaceControlEvent                     `json:"raceControl"`
	Weather     []RaceWeatherSample                    `json:"weather"`
}
