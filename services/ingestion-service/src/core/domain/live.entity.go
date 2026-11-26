/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live.entity.go - Live ingestion job models: session, status and polled resources.
##
*/

package domain

import "time"

// LiveState is the state of a live ingestion job.
type LiveState string

const (
	// LiveWaiting: started ahead of the session, polling begins LiveLead before its start.
	LiveWaiting LiveState = "waiting"
	LiveRunning LiveState = "running"
	// LiveStopping: stop requested, the current poll is being interrupted.
	LiveStopping LiveState = "stopping"
	// LiveStopped, LiveCompleted and LiveFailed are final: stopped on request, session over, too
	// many consecutive failures or a resource still failing at the end.
	LiveStopped   LiveState = "stopped"
	LiveCompleted LiveState = "completed"
	LiveFailed    LiveState = "failed"
)

// Final reports whether the job is over.
func (s LiveState) Final() bool {
	return s == LiveStopped || s == LiveCompleted || s == LiveFailed
}

// LiveLead is how long before the session start OpenF1 begins to publish its live data.
const LiveLead = 30 * time.Minute

// HeavyHistoryLead is how far before the session start the windowed resources (car_data,
// location) begin: OpenF1 keeps the cars' laps to the grid, up to about 50 minutes before the
// start, in the session's data.
const HeavyHistoryLead = time.Hour

// LiveSession is the OpenF1 session a live job follows.
type LiveSession struct {
	SessionKey int
	MeetingKey int
	StartUTC   time.Time
	EndUTC     time.Time
}

// LiveResource describes how one OpenF1 resource is polled.
type LiveResource struct {
	Name string
	// DateField is the OpenF1 field filtered by time; empty when the resource has none, and is
	// then fetched whole (it is small).
	DateField string
	// Every is the number of poll intervals between two polls.
	Every int
	// Overlap is how far before the latest known row each poll starts again: rows published
	// late, or still changing.
	Overlap time.Duration
	// Window bounds the period of one call for the heavy resources; zero: no upper bound.
	Window time.Duration
	// Refresh fetches the whole resource once more when the session ends (rows completed after
	// the overlap, or without a date).
	Refresh bool
}

// LiveResources is the poll schedule. With the default 3 s interval it makes 54 calls a minute,
// under the 60 of an OpenF1 subscription; the resources the overlay needs most come first.
var LiveResources = []LiveResource{
	{Name: "race_control", DateField: "date", Every: 2, Overlap: 30 * time.Second},
	{Name: "position", DateField: "date", Every: 2, Overlap: 10 * time.Second},
	{Name: "location", DateField: "date", Every: 2, Overlap: 10 * time.Second, Window: 5 * time.Minute},
	{Name: "car_data", DateField: "date", Every: 2, Overlap: 10 * time.Second, Window: 5 * time.Minute},
	{Name: "intervals", DateField: "date", Every: 4, Overlap: 10 * time.Second},
	{Name: "laps", DateField: "date_start", Every: 5, Overlap: 5 * time.Minute, Refresh: true},
	{Name: "pit", DateField: "date", Every: 10, Overlap: 5 * time.Minute, Refresh: true},
	{Name: "team_radio", DateField: "date", Every: 20, Overlap: 5 * time.Minute},
	{Name: "stints", Every: 20, Refresh: true},
	{Name: "weather", DateField: "date", Every: 20, Overlap: 2 * time.Minute},
}

// LiveWindow is the period of one poll. A nil bound is open.
type LiveWindow struct {
	From *time.Time
	To   *time.Time
}

// LiveResourceStatus reports one resource of a live job.
type LiveResourceStatus struct {
	Resource         string     `json:"resource"`
	RowCount         int        `json:"rowCount"`
	LatestRowAtUTC   *time.Time `json:"latestRowAtUtc"`
	LastSuccessAtUTC *time.Time `json:"lastSuccessAtUtc"`
	// Error is the short reason of the latest failure, empty once a poll succeeds again.
	Error string `json:"error,omitempty"`
}

// LiveStatus reports a live job. Error is the short reason of a failed job or catalog step.
type LiveStatus struct {
	SessionKey          int                  `json:"sessionKey"`
	SessionID           string               `json:"sessionId"`
	MeetingKey          int                  `json:"meetingKey"`
	State               LiveState            `json:"state"`
	SessionStartUTC     time.Time            `json:"sessionStartUtc"`
	SessionEndUTC       time.Time            `json:"sessionEndUtc"`
	StartedAtUTC        time.Time            `json:"startedAtUtc"`
	EndedAtUTC          *time.Time           `json:"endedAtUtc"`
	ConsecutiveFailures int                  `json:"consecutiveFailures"`
	CatalogIngested     bool                 `json:"catalogIngested"`
	Error               string               `json:"error,omitempty"`
	Resources           []LiveResourceStatus `json:"resources"`
}
