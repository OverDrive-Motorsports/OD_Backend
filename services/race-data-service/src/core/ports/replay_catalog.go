/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog.go - Ports of the replay catalog (GET /championships/{code}/replays).
##
*/

package ports

import (
	"context"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
)

// ChampionshipSessionRow is one session of a championship with its event context, as returned
// by championship-service's GET /championships/{code}/sessions.
type ChampionshipSessionRow struct {
	SessionID  string    `json:"sessionId"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	StartTime  time.Time `json:"startTime"`
	EventName  string    `json:"eventName"`
	SeasonYear int       `json:"seasonYear"`
	Circuit    string    `json:"circuit"`
}

// ChampionshipSessionCatalog lists a championship's sessions from championship-service.
// It returns (nil, nil) when the championship code is unknown. Kept separate from
// ChampionshipClient so existing consumers and their fakes are untouched.
type ChampionshipSessionCatalog interface {
	ListChampionshipSessions(ctx context.Context, code string) ([]ChampionshipSessionRow, error)
}

// ReplayCatalogRepository tells which sessions have a playable replay in this service's store.
type ReplayCatalogRepository interface {
	// SessionsWithReplay returns the subset of sessionIDs that have both telemetry and
	// location samples, as a set.
	SessionsWithReplay(ctx context.Context, sessionIDs []string) (map[string]struct{}, error)
}

// ReplayCatalogUseCase exposes the GET /championships/{code}/replays application logic.
type ReplayCatalogUseCase interface {
	// ListReplays returns the championship's playable replays, newest first; (nil, nil) when
	// the championship is unknown, an empty list when it has no replay.
	ListReplays(ctx context.Context, championshipCode string) ([]domain.ReplayCatalogEntry, error)
}
