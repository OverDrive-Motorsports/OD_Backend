/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_catalog.go - Prisma-backed catalog reads and archive reconstruction helpers.
##
*/

package prisma

import (
	"context"
	"fmt"
	"strings"
	"time"

	"overdrive/internal/domain"
	"overdrive/resources/db"
)

func (s *RaceArchiveStore) ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error) {
	items, err := s.client.Championship.FindMany().
		OrderBy(db.Championship.Name.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find championships: %w", err)
	}

	out := make([]domain.ChampionshipSummary, 0, len(items))
	for i := range items {
		out = append(out, championshipSummaryFromModel(&items[i]))
	}
	return out, nil
}

// GetChampionshipEvents returns one championship and its events ordered by schedule.
func (s *RaceArchiveStore) GetChampionshipEvents(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.EventSummary, bool, error) {
	championship, err := s.client.Championship.FindFirst(
		db.Championship.Code.Equals(strings.ToLower(strings.TrimSpace(code))),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.ChampionshipSummary{}, nil, false, nil
		}
		return domain.ChampionshipSummary{}, nil, false, fmt.Errorf("find championship: %w", err)
	}

	events, err := s.client.Event.FindMany(
		db.Event.ChampionshipID.Equals(championship.ID),
	).
		OrderBy(
			db.Event.SeasonYear.Order(db.SortOrderDesc),
			db.Event.StartTimeUtc.Order(db.SortOrderAsc),
		).
		Exec(ctx)
	if err != nil {
		return domain.ChampionshipSummary{}, nil, false, fmt.Errorf("find events: %w", err)
	}

	out := make([]domain.EventSummary, 0, len(events))
	for i := range events {
		out = append(out, eventSummaryFromModel(&events[i]))
	}

	return championshipSummaryFromModel(championship), out, true, nil
}

// GetEvent returns one stored event by identifier.
func (s *RaceArchiveStore) GetEvent(ctx context.Context, eventID string) (domain.EventSummary, bool, error) {
	event, err := s.client.Event.FindUnique(
		db.Event.ID.Equals(strings.TrimSpace(eventID)),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.EventSummary{}, false, nil
		}
		return domain.EventSummary{}, false, fmt.Errorf("find event: %w", err)
	}

	return eventSummaryFromModel(event), true, nil
}

// ListEventSessions returns the stored sessions for one event ordered by start time.
func (s *RaceArchiveStore) ListEventSessions(ctx context.Context, eventID string) ([]domain.SessionSummary, bool, error) {
	if _, err := s.client.Event.FindUnique(
		db.Event.ID.Equals(strings.TrimSpace(eventID)),
	).Exec(ctx); err != nil {
		if db.IsErrNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("find event: %w", err)
	}

	sessions, err := s.client.Session.FindMany(
		db.Session.EventID.Equals(eventID),
	).
		OrderBy(
			db.Session.StartedAtUtc.Order(db.SortOrderAsc),
		).
		Exec(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("find sessions: %w", err)
	}

	out := make([]domain.SessionSummary, 0, len(sessions))
	for i := range sessions {
		out = append(out, s.sessionSummaryFromModel(ctx, &sessions[i]))
	}

	return out, true, nil
}

// GetSession returns one stored session by identifier.
func (s *RaceArchiveStore) GetSession(ctx context.Context, sessionID string) (domain.SessionSummary, bool, error) {
	session, err := s.client.Session.FindUnique(
		db.Session.ID.Equals(strings.TrimSpace(sessionID)),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.SessionSummary{}, false, nil
		}
		return domain.SessionSummary{}, false, fmt.Errorf("find session: %w", err)
	}

	return s.sessionSummaryFromModel(ctx, session), true, nil
}

// GetSessionDriverBroadcast returns the stored broadcast URL for one driver in one session.
func (s *RaceArchiveStore) GetSessionDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (string, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || driverNumber <= 0 {
		return "", false, nil
	}

	items, err := s.client.SessionDriverBroadcast.FindMany(
		db.SessionDriverBroadcast.SessionID.Equals(sessionID),
	).Exec(ctx)
	if err != nil {
		return "", false, fmt.Errorf("find session driver broadcasts: %w", err)
	}

	for _, item := range items {
		driver, driverErr := s.client.Driver.FindUnique(
			db.Driver.ID.Equals(item.DriverID),
		).Exec(ctx)
		if driverErr != nil {
			if db.IsErrNotFound(driverErr) {
				continue
			}
			return "", false, fmt.Errorf("find driver for session broadcast: %w", driverErr)
		}
		if driver.Number != driverNumber {
			continue
		}

		return optionalStringValue(item.BroadcastURL), true, nil
	}

	return "", false, nil
}

// archiveFromModel decodes one persisted archive row into the domain payload.
func (s *RaceArchiveStore) archiveFromModel(ctx context.Context, stored *db.RaceArchiveModel) (domain.RaceArchive, error) {
	if stored == nil {
		return domain.RaceArchive{}, fmt.Errorf("race archive model is nil")
	}

	envelope, err := unmarshalMetadataEnvelope(stored.Metadata)
	if err != nil {
		return domain.RaceArchive{}, fmt.Errorf("decode metadata envelope: %w", err)
	}

	counts := map[string]int{}
	if err := unmarshalPrismaJSON(stored.Counts, &counts); err != nil {
		return domain.RaceArchive{}, fmt.Errorf("decode counts: %w", err)
	}

	fetchErrors := map[string]string{}
	if rawFetchErrors, ok := stored.FetchErrors(); ok {
		if err := unmarshalPrismaJSON(rawFetchErrors, &fetchErrors); err != nil {
			return domain.RaceArchive{}, fmt.Errorf("decode fetch_errors: %w", err)
		}
	}

	chunks, err := s.client.RaceDatasetChunk.FindMany(
		db.RaceDatasetChunk.ArchiveID.Equals(stored.ID),
	).
		OrderBy(
			db.RaceDatasetChunk.Dataset.Order(db.SortOrderAsc),
			db.RaceDatasetChunk.ChunkIndex.Order(db.SortOrderAsc),
		).
		Exec(ctx)
	if err != nil {
		return domain.RaceArchive{}, fmt.Errorf("find race dataset chunks: %w", err)
	}

	datasets := make(map[string][]map[string]any, len(chunks))
	for _, chunk := range chunks {
		rows := []map[string]any{}
		if err := unmarshalPrismaJSON(chunk.Rows, &rows); err != nil {
			return domain.RaceArchive{}, fmt.Errorf("decode dataset %q chunk %d: %w", chunk.Dataset, chunk.ChunkIndex, err)
		}
		datasets[chunk.Dataset] = append(datasets[chunk.Dataset], rows...)
	}

	metadata := envelope.Metadata
	if metadata.GeneratedAt.IsZero() {
		metadata.GeneratedAt = stored.GeneratedAt
	}

	archive := domain.RaceArchive{
		Metadata:    metadata,
		Meeting:     nonNilMap(envelope.Meeting),
		AllSessions: nonNilRows(envelope.AllSessions),
		RaceSession: nonNilMap(envelope.RaceSession),
		Datasets:    datasets,
		Counts:      counts,
	}
	if len(fetchErrors) > 0 {
		archive.FetchErrors = fetchErrors
	}

	return archive, nil
}

// mergeArchiveModels builds one merged archive for the same session across several imports.
func (s *RaceArchiveStore) mergeArchiveModels(ctx context.Context, models []db.RaceArchiveModel) (domain.RaceArchive, error) {
	merged := domain.RaceArchive{
		Meeting:     map[string]any{},
		AllSessions: []map[string]any{},
		RaceSession: map[string]any{},
		Datasets:    map[string][]map[string]any{},
		Counts:      map[string]int{},
		FetchErrors: map[string]string{},
	}

	rowSeen := map[string]map[string]struct{}{}
	allSessionSeen := map[string]struct{}{}
	driverNumbers := map[int]struct{}{}
	hasFullArchive := false

	for i := range models {
		current, err := s.archiveFromModel(ctx, &models[i])
		if err != nil {
			return domain.RaceArchive{}, err
		}

		merged.Metadata = current.Metadata
		if len(current.Meeting) > 0 {
			merged.Meeting = cloneMap(current.Meeting)
		}
		if len(current.RaceSession) > 0 {
			merged.RaceSession = cloneMap(current.RaceSession)
		}
		if len(current.FetchErrors) > 0 {
			for key, value := range current.FetchErrors {
				merged.FetchErrors[key] = value
			}
		}

		if current.Metadata.DriverNumber > 0 {
			driverNumbers[current.Metadata.DriverNumber] = struct{}{}
		} else {
			hasFullArchive = true
		}

		merged.AllSessions = mergeUniqueRows(merged.AllSessions, current.AllSessions, allSessionSeen)

		for dataset, rows := range current.Datasets {
			seen := rowSeen[dataset]
			if seen == nil {
				seen = map[string]struct{}{}
				rowSeen[dataset] = seen
			}
			merged.Datasets[dataset] = mergeUniqueRows(merged.Datasets[dataset], rows, seen)
		}
	}

	if len(merged.FetchErrors) == 0 {
		merged.FetchErrors = nil
	}
	if hasFullArchive || len(driverNumbers) > 1 {
		merged.Metadata.DriverNumber = 0
	}

	for dataset, rows := range merged.Datasets {
		merged.Counts[dataset] = len(rows)
	}

	return merged, nil
}

// championshipSummaryFromModel maps a Prisma championship model to its API summary.
func championshipSummaryFromModel(model *db.ChampionshipModel) domain.ChampionshipSummary {
	if model == nil {
		return domain.ChampionshipSummary{}
	}

	category, _ := model.Category()

	return domain.ChampionshipSummary{
		ID:        model.ID,
		Code:      model.Code,
		Name:      model.Name,
		Category:  optionalStringValue(category),
		IsActive:  model.IsActive,
		CreatedAt: time.Time(model.CreatedAt).UTC(),
		UpdatedAt: time.Time(model.UpdatedAt).UTC(),
	}
}

// eventSummaryFromModel maps a Prisma event model to its API summary.
func eventSummaryFromModel(model *db.EventModel) domain.EventSummary {
	if model == nil {
		return domain.EventSummary{}
	}

	officialName, _ := model.OfficialName()
	countryName, _ := model.CountryName()
	countryCode, _ := model.CountryCode()
	circuitName, _ := model.CircuitName()
	externalKey, _ := model.ExternalKey()
	roundNumber, _ := model.RoundNumber()
	var endsAt *time.Time
	parsedEnd := time.Time(model.EndTimeUtc).UTC()
	endsAt = &parsedEnd

	return domain.EventSummary{
		ID:             model.ID,
		ChampionshipID: model.ChampionshipID,
		SeasonYear:     model.SeasonYear,
		RoundNumber:    optionalIntPointer(roundNumber),
		Name:           model.Name,
		OfficialName:   optionalStringValue(officialName),
		CountryName:    optionalStringValue(countryName),
		CountryCode:    optionalStringValue(countryCode),
		CircuitName:    optionalStringValue(circuitName),
		ExternalKey:    optionalStringValue(externalKey),
		Status:         string(model.Status),
		StartsAtUTC:    time.Time(model.StartTimeUtc).UTC(),
		EndsAtUTC:      endsAt,
		CreatedAt:      time.Time(model.CreatedAt).UTC(),
		UpdatedAt:      time.Time(model.UpdatedAt).UTC(),
	}
}

// sessionSummaryFromModel maps a Prisma session model to its API summary.
func (s *RaceArchiveStore) sessionSummaryFromModel(ctx context.Context, model *db.SessionModel) domain.SessionSummary {
	if model == nil {
		return domain.SessionSummary{}
	}

	name, _ := model.Name()
	externalKey, _ := model.ExternalKey()
	broadcastURL, _ := model.BroadcastURL()
	var endedAt *time.Time
	if value, ok := model.EndedAtUtc(); ok {
		parsed := time.Time(value).UTC()
		endedAt = &parsed
	}

	summary := domain.SessionSummary{
		ID:           model.ID,
		EventID:      model.EventID,
		Type:         string(model.Type),
		Status:       string(model.Status),
		Name:         optionalStringValue(name),
		ExternalKey:  optionalStringValue(externalKey),
		BroadcastURL: optionalStringValue(broadcastURL),
		StartedAtUTC: time.Time(model.StartedAtUtc).UTC(),
		EndedAtUTC:   endedAt,
		CreatedAt:    time.Time(model.CreatedAt).UTC(),
		UpdatedAt:    time.Time(model.UpdatedAt).UTC(),
	}

	latestArchive, err := s.client.RaceArchive.FindFirst(
		db.RaceArchive.SessionID.Equals(model.ID),
	).
		OrderBy(db.RaceArchive.GeneratedAt.Order(db.SortOrderDesc)).
		Exec(ctx)
	if err == nil {
		summary.LatestCounts = map[string]int{}
		if decodeErr := unmarshalPrismaJSON(latestArchive.Counts, &summary.LatestCounts); decodeErr != nil {
			summary.LatestCounts = nil
		}
		storedAt := time.Time(latestArchive.GeneratedAt).UTC()
		summary.LatestStored = &storedAt
	}

	return summary
}
