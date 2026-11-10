/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog.entity.go - Replay catalog entry exposed by GET /championships/{code}/replays.
##
*/

package domain

import "time"

// ReplayCatalogEntry is one playable replay of a championship: a session whose
// GET /sessions/{sessionId}/race/replay dump carries car motion (telemetry + x/y/z
// location samples). SessionID is the key the client then passes to that replay route.
type ReplayCatalogEntry struct {
	SessionID  string    `json:"sessionId"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	StartTime  time.Time `json:"startTime"`
	SeasonYear int       `json:"seasonYear"`
	EventName  string    `json:"eventName"`
	Circuit    string    `json:"circuit"`
}
