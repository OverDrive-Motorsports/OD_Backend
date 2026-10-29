/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog.usecase.go - Lists the playable replays of a championship.
##
*/

package usecases

import (
	"context"
	"fmt"
	"sort"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
)

type ReplayCatalogUseCase struct {
	sessions   ports.ChampionshipSessionCatalog
	repository ports.ReplayCatalogRepository
}

// NewReplayCatalogUseCase builds and returns a replay catalog use case with its required dependencies.
func NewReplayCatalogUseCase(sessions ports.ChampionshipSessionCatalog, repository ports.ReplayCatalogRepository) *ReplayCatalogUseCase {
	return &ReplayCatalogUseCase{sessions: sessions, repository: repository}
}

// ListReplays lists the championship's sessions from championship-service, keeps only those
// with a playable replay in the local store, and returns them newest first. A championship
// unknown to championship-service yields (nil, nil); a downstream failure is wrapped in
// domain.ErrChampionshipUnavailable so the controller can answer 502 rather than 500.
func (u *ReplayCatalogUseCase) ListReplays(ctx context.Context, championshipCode string) ([]domain.ReplayCatalogEntry, error) {
	sessions, err := u.sessions.ListChampionshipSessions(ctx, championshipCode)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrChampionshipUnavailable, err)
	}
	if sessions == nil {
		return nil, nil
	}
	entries := []domain.ReplayCatalogEntry{}
	if len(sessions) == 0 {
		return entries, nil
	}
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.SessionID)
	}
	playable, err := u.repository.SessionsWithReplay(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		if _, ok := playable[session.SessionID]; !ok {
			continue
		}
		entries = append(entries, domain.ReplayCatalogEntry{
			SessionID:  session.SessionID,
			Name:       session.Name,
			Type:       session.Type,
			StartTime:  session.StartTime,
			SeasonYear: session.SeasonYear,
			EventName:  session.EventName,
			Circuit:    session.Circuit,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].StartTime.After(entries[j].StartTime) })
	return entries, nil
}
