/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_live.entity.go - Package domain source file for services/race-data-service/src/core/domain.
##
*/

package domain

import "time"

// NOTE: all response field names are camelCase — this is the public contract
// consumed by the gateway's /v1/race-data/* routes (AR/mobile clients).

// RacePosition represents a driver's live race position, gaps, and lap count.
type RacePosition struct {
	DriverNumber  int       `json:"driverNumber"`
	Position      int       `json:"position"`
	GapToLeader   string    `json:"gapToLeader,omitempty"`
	GapAhead      *string   `json:"gapAhead"`
	GapBehind     *string   `json:"gapBehind"`
	LapsCompleted int       `json:"lapsCompleted"`
	Timestamp     time.Time `json:"timestamp"`
}

// RaceLapEntry is a single lap's timing breakdown.
type RaceLapEntry struct {
	LapNumber   int      `json:"lapNumber"`
	LapDuration *float64 `json:"lapDuration"`
	Sector1     *float64 `json:"sector1"`
	Sector2     *float64 `json:"sector2"`
	Sector3     *float64 `json:"sector3"`
	IsPitOutLap bool     `json:"isPitOutLap"`
}

// RaceLapsResponse groups a driver's laps with computed best/average lap times.
type RaceLapsResponse struct {
	DriverNumber int            `json:"driverNumber"`
	Laps         []RaceLapEntry `json:"laps"`
	BestLap      *float64       `json:"bestLap"`
	AverageLap   *float64       `json:"averageLap"`
}

// RaceStint represents one tyre stint for a driver.
type RaceStint struct {
	DriverNumber   int    `json:"driverNumber"`
	StintNumber    int    `json:"stintNumber"`
	Compound       string `json:"compound,omitempty"`
	LapStart       int    `json:"lapStart"`
	LapEnd         int    `json:"lapEnd"`
	TyreAgeAtStart int    `json:"tyreAgeAtStart"`
}

// RacePitStop represents a single pit stop.
type RacePitStop struct {
	DriverNumber int     `json:"driverNumber"`
	LapNumber    int     `json:"lapNumber"`
	PitDuration  float64 `json:"pitDuration"`
}

// RaceControlPenalty is the optional penalty payload attached to a race control event.
// NOTE: OpenF1 does not expose structured penalty data today, so these fields are
// always nil until a data source is wired in (see doc/endpoint.md gap note).
type RaceControlPenalty struct {
	DriverNumber *int    `json:"driverNumber"`
	TimePenalty  *string `json:"timePenalty"`
}

// RaceControlEvent represents a flag, safety car, penalty, or track alert.
type RaceControlEvent struct {
	Category  string              `json:"category,omitempty"`
	Flag      string              `json:"flag,omitempty"`
	Penalty   *RaceControlPenalty `json:"penality"`
	SafetyCar *string             `json:"safetyCar"`
	Message   string              `json:"message,omitempty"`
	LapNumber int                 `json:"lapNumber"`
	Timestamp time.Time           `json:"timestamp"`
}

// RaceWeatherSample is a single weather reading for the session.
type RaceWeatherSample struct {
	Timestamp        time.Time `json:"timestamp"`
	AirTemperature   *float64  `json:"airTemperature"`
	TrackTemperature *float64  `json:"trackTemperature"`
	Humidity         *float64  `json:"humidity"`
	WindSpeed        *float64  `json:"windSpeed"`
	Rainfall         bool      `json:"rainfall"`
}

// RaceRadioMessage is a team radio message reference.
type RaceRadioMessage struct {
	DriverNumber int       `json:"driverNumber"`
	Timestamp    time.Time `json:"timestamp"`
	AudioURL     string    `json:"audioUrl,omitempty"`
}
