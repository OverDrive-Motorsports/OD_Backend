/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_dump.entity.go - Pieces of the streamed replay dump, its query and its limits.
##
*/

package domain

import (
	"encoding/json"
	"errors"
	"time"
)

// ReplayDataset is one top-level key of the GET /race/replay dump. A driver-scoped dataset is a
// JSON object keyed by driver number; a track-wide one is a flat array.
type ReplayDataset struct {
	Name         string
	DriverScoped bool
}

// ReplayDatasets lists the dump keys in the order they are written, the field order of
// RaceReplay (the shape clients have always received).
var ReplayDatasets = []ReplayDataset{
	{Name: "telemetry", DriverScoped: true},
	{Name: "position", DriverScoped: true},
	{Name: "radio", DriverScoped: true},
	{Name: "pitStop", DriverScoped: true},
	{Name: "stint", DriverScoped: true},
	{Name: "lap", DriverScoped: true},
	{Name: "raceControl", DriverScoped: false},
	{Name: "weather", DriverScoped: false},
}

// ReplayPart is one piece of the dump, small enough to hold in memory: the samples of one driver
// for a driver-scoped dataset, or the whole array of a track-wide dataset. Rows is a JSON array.
// Parts are produced in ReplayDatasets order and, within a dataset, by driver key in string
// order (the order encoding/json gives map keys).
type ReplayPart struct {
	Dataset      string
	DriverNumber *int
	Rows         json.RawMessage
}

// TimeWindow restricts the replay to samples whose timestamp is within [From, To]; a nil bound
// is open.
type TimeWindow struct {
	From *time.Time
	To   *time.Time
}

// IsZero reports whether the window keeps every sample.
func (w TimeWindow) IsZero() bool {
	return w.From == nil && w.To == nil
}

// Contains reports whether t is inside the window.
func (w TimeWindow) Contains(t time.Time) bool {
	if w.From != nil && t.Before(*w.From) {
		return false
	}
	if w.To != nil && t.After(*w.To) {
		return false
	}
	return true
}

// ReplayQuery selects what GET /race/replay returns: one driver or all, and an optional window.
type ReplayQuery struct {
	DriverNumber *int
	Window       TimeWindow
}

// ErrReplayBusy is returned when the maximum number of concurrent replays is already running
// (503, the client retries later).
var ErrReplayBusy = errors.New("too many replays in progress")

// ErrInvalidTimeWindow is returned when fromUtc is after toUtc (400).
var ErrInvalidTimeWindow = errors.New("fromUtc must not be after toUtc")

// DefaultReplayMaxConcurrent is how many replays one instance builds at the same time, and
// DefaultReplayCacheMaxConcurrent how many cached replays it serves at the same time (each holds
// one chunk, a few MB, while a possibly slow client reads it).
const (
	DefaultReplayMaxConcurrent      = 4
	DefaultReplayCacheMaxConcurrent = 32
)
