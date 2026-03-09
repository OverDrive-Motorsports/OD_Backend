/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_store.go - Prisma-backed repository for persistent race archive storage and retrieval.
##
*/

package prisma

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"overdrive/internal/domain"
	"overdrive/resources/db"
)

const (
	providerCodeOpenF1     = "openf1"
	providerNameOpenF1     = "OpenF1"
	championshipCodeF1     = "f1"
	championshipNameF1     = "Formula 1"
	championshipCategoryF1 = "single-seater"
)

type RaceArchiveStore struct {
	client *db.PrismaClient
}

type metadataEnvelope struct {
	Metadata    domain.Metadata  `json:"metadata"`
	Meeting     map[string]any   `json:"meeting"`
	AllSessions []map[string]any `json:"all_sessions"`
	RaceSession map[string]any   `json:"race_session"`
}

// NewRaceArchiveStore builds a Prisma-backed race archive repository.
func NewRaceArchiveStore(client *db.PrismaClient) *RaceArchiveStore {
	return &RaceArchiveStore{client: client}
}

// Store persists one fetched race archive in PostgreSQL.
func (s *RaceArchiveStore) Store(ctx context.Context, archive domain.RaceArchive) (time.Time, error) {
	if s.client == nil {
		return time.Time{}, fmt.Errorf("prisma client is nil")
	}

	provider, err := s.ensureProvider(ctx)
	if err != nil {
		return time.Time{}, err
	}

	event, err := s.ensureEvent(ctx, provider.ID, archive)
	if err != nil {
		return time.Time{}, err
	}

	championship, err := s.ensureChampionship(ctx, provider.ID)
	if err != nil {
		return time.Time{}, err
	}

	race, err := s.ensureRace(ctx, championship.ID, event.ID, archive)
	if err != nil {
		return time.Time{}, err
	}

	session, err := s.ensureSession(ctx, event.ID, race.ID, archive)
	if err != nil {
		return time.Time{}, err
	}

	if err := s.syncParticipants(ctx, provider.ID, archive.Datasets["drivers"]); err != nil {
		return time.Time{}, err
	}

	metadataJSON, err := marshalPrismaJSON(metadataEnvelope{
		Metadata:    archive.Metadata,
		Meeting:     archive.Meeting,
		AllSessions: archive.AllSessions,
		RaceSession: archive.RaceSession,
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("marshal metadata envelope: %w", err)
	}

	countsJSON, err := marshalPrismaJSON(archive.Counts)
	if err != nil {
		return time.Time{}, fmt.Errorf("marshal counts: %w", err)
	}

	generatedAt := archive.Metadata.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	archiveParams := []db.RaceArchiveSetParam{
		db.RaceArchive.Mode.Set(resolveArchiveMode(archive.Metadata.DriverNumber)),
		db.RaceArchive.GeneratedAt.Set(generatedAt),
		db.RaceArchive.SourceMeetingKey.SetIfPresent(optionalInt(archive.Metadata.MeetingKey)),
		db.RaceArchive.SourceSessionKey.SetIfPresent(optionalInt(archive.Metadata.RaceSessKey)),
		db.RaceArchive.SourceDriverNumber.SetIfPresent(optionalInt(archive.Metadata.DriverNumber)),
	}

	if len(archive.FetchErrors) > 0 {
		fetchErrorsJSON, err := marshalPrismaJSON(archive.FetchErrors)
		if err != nil {
			return time.Time{}, fmt.Errorf("marshal fetch_errors: %w", err)
		}
		archiveParams = append(archiveParams, db.RaceArchive.FetchErrors.Set(fetchErrorsJSON))
	}

	createdArchive, err := s.client.RaceArchive.CreateOne(
		db.RaceArchive.Metadata.Set(metadataJSON),
		db.RaceArchive.Counts.Set(countsJSON),
		db.RaceArchive.Session.Link(db.Session.ID.Equals(session.ID)),
		archiveParams...,
	).Exec(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("create race archive: %w", err)
	}

	if err := s.storeDatasets(ctx, createdArchive.ID, archive.Datasets); err != nil {
		return time.Time{}, err
	}

	if err := s.storeNormalizedSessionData(ctx, session.ID, archive); err != nil {
		return time.Time{}, err
	}

	return createdArchive.GeneratedAt, nil
}

// GetLatest returns the latest persisted race archive from PostgreSQL.
func (s *RaceArchiveStore) GetLatest(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	if s.client == nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("prisma client is nil")
	}

	latest, err := s.client.RaceArchive.FindFirst().
		OrderBy(db.RaceArchive.GeneratedAt.Order(db.SortOrderDesc)).
		Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.RaceArchive{}, time.Time{}, false, nil
		}
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("find latest race archive: %w", err)
	}

	archive, err := s.archiveFromModel(ctx, latest)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, err
	}

	return archive, latest.GeneratedAt, true, nil
}

// GetLatestMerged returns a merged archive view across all imports for the latest stored session.
func (s *RaceArchiveStore) GetLatestMerged(ctx context.Context) (domain.RaceArchive, time.Time, bool, error) {
	if s.client == nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("prisma client is nil")
	}

	latest, err := s.client.RaceArchive.FindFirst().
		OrderBy(db.RaceArchive.GeneratedAt.Order(db.SortOrderDesc)).
		Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.RaceArchive{}, time.Time{}, false, nil
		}
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("find latest race archive: %w", err)
	}

	archives, err := s.client.RaceArchive.FindMany(
		db.RaceArchive.SessionID.Equals(latest.SessionID),
	).
		OrderBy(db.RaceArchive.GeneratedAt.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, fmt.Errorf("find session race archives: %w", err)
	}

	merged, err := s.mergeArchiveModels(ctx, archives)
	if err != nil {
		return domain.RaceArchive{}, time.Time{}, false, err
	}

	return merged, latest.GeneratedAt, true, nil
}

// ListChampionships returns all stored championships ordered by name.
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

// GetChampionshipRaces returns one championship and its races ordered by schedule.
func (s *RaceArchiveStore) GetChampionshipRaces(ctx context.Context, code string) (domain.ChampionshipSummary, []domain.RaceSummary, bool, error) {
	championship, err := s.client.Championship.FindFirst(
		db.Championship.Code.Equals(strings.ToLower(strings.TrimSpace(code))),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.ChampionshipSummary{}, nil, false, nil
		}
		return domain.ChampionshipSummary{}, nil, false, fmt.Errorf("find championship: %w", err)
	}

	races, err := s.client.Race.FindMany(
		db.Race.ChampionshipID.Equals(championship.ID),
	).
		OrderBy(
			db.Race.SeasonYear.Order(db.SortOrderDesc),
			db.Race.StartsAtUtc.Order(db.SortOrderAsc),
		).
		Exec(ctx)
	if err != nil {
		return domain.ChampionshipSummary{}, nil, false, fmt.Errorf("find races: %w", err)
	}

	out := make([]domain.RaceSummary, 0, len(races))
	for i := range races {
		out = append(out, raceSummaryFromModel(&races[i]))
	}

	return championshipSummaryFromModel(championship), out, true, nil
}

// GetRace returns one stored race by identifier.
func (s *RaceArchiveStore) GetRace(ctx context.Context, raceID string) (domain.RaceSummary, bool, error) {
	race, err := s.client.Race.FindUnique(
		db.Race.ID.Equals(strings.TrimSpace(raceID)),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.RaceSummary{}, false, nil
		}
		return domain.RaceSummary{}, false, fmt.Errorf("find race: %w", err)
	}

	return raceSummaryFromModel(race), true, nil
}

// ListRaceSessions returns the stored sessions for one race ordered by start time.
func (s *RaceArchiveStore) ListRaceSessions(ctx context.Context, raceID string) ([]domain.SessionSummary, bool, error) {
	if _, err := s.client.Race.FindUnique(
		db.Race.ID.Equals(strings.TrimSpace(raceID)),
	).Exec(ctx); err != nil {
		if db.IsErrNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("find race: %w", err)
	}

	sessions, err := s.client.Session.FindMany(
		db.Session.RaceID.Equals(raceID),
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

// raceSummaryFromModel maps a Prisma race model to its API summary.
func raceSummaryFromModel(model *db.RaceModel) domain.RaceSummary {
	if model == nil {
		return domain.RaceSummary{}
	}

	roundNumber, _ := model.RoundNumber()
	officialName, _ := model.OfficialName()
	countryName, _ := model.CountryName()
	countryCode, _ := model.CountryCode()
	circuitName, _ := model.CircuitName()
	externalKey, _ := model.ExternalKey()
	var endsAt *time.Time
	if value, ok := model.EndsAtUtc(); ok {
		parsed := time.Time(value).UTC()
		endsAt = &parsed
	}

	return domain.RaceSummary{
		ID:           model.ID,
		Championship: model.ChampionshipID,
		EventID:      model.EventID,
		SeasonYear:   model.SeasonYear,
		RoundNumber:  optionalIntPointer(roundNumber),
		Name:         model.Name,
		OfficialName: optionalStringValue(officialName),
		CountryName:  optionalStringValue(countryName),
		CountryCode:  optionalStringValue(countryCode),
		CircuitName:  optionalStringValue(circuitName),
		ExternalKey:  optionalStringValue(externalKey),
		Status:       string(model.Status),
		StartsAtUTC:  time.Time(model.StartsAtUtc).UTC(),
		EndsAtUTC:    endsAt,
		CreatedAt:    time.Time(model.CreatedAt).UTC(),
		UpdatedAt:    time.Time(model.UpdatedAt).UTC(),
	}
}

// sessionSummaryFromModel maps a Prisma session model to its API summary.
func (s *RaceArchiveStore) sessionSummaryFromModel(ctx context.Context, model *db.SessionModel) domain.SessionSummary {
	if model == nil {
		return domain.SessionSummary{}
	}

	raceID, _ := model.RaceID()
	name, _ := model.Name()
	externalKey, _ := model.ExternalKey()
	var endedAt *time.Time
	if value, ok := model.EndedAtUtc(); ok {
		parsed := time.Time(value).UTC()
		endedAt = &parsed
	}

	summary := domain.SessionSummary{
		ID:           model.ID,
		EventID:      model.EventID,
		RaceID:       optionalStringValue(raceID),
		Type:         string(model.Type),
		Status:       string(model.Status),
		Name:         optionalStringValue(name),
		ExternalKey:  optionalStringValue(externalKey),
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

// ensureProvider returns the OpenF1 provider row, creating it on first use.
func (s *RaceArchiveStore) ensureProvider(ctx context.Context) (*db.ProviderModel, error) {
	provider, err := s.client.Provider.FindFirst(
		db.Provider.Code.Equals(providerCodeOpenF1),
	).Exec(ctx)
	if err == nil {
		return provider, nil
	}
	if !db.IsErrNotFound(err) {
		return nil, fmt.Errorf("find provider: %w", err)
	}

	provider, err = s.client.Provider.CreateOne(
		db.Provider.Code.Set(providerCodeOpenF1),
		db.Provider.Name.Set(providerNameOpenF1),
		db.Provider.Status.Set(db.ProviderStatusActive),
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return provider, nil
}

// ensureChampionship returns the active championship row, creating it on first use.
func (s *RaceArchiveStore) ensureChampionship(ctx context.Context, providerID string) (*db.ChampionshipModel, error) {
	championship, err := s.client.Championship.FindFirst(
		db.Championship.ProviderID.Equals(providerID),
		db.Championship.Code.Equals(championshipCodeF1),
	).Exec(ctx)
	if err == nil {
		return championship, nil
	}
	if !db.IsErrNotFound(err) {
		return nil, fmt.Errorf("find championship: %w", err)
	}

	championship, err = s.client.Championship.CreateOne(
		db.Championship.Code.Set(championshipCodeF1),
		db.Championship.Name.Set(championshipNameF1),
		db.Championship.Provider.Link(db.Provider.ID.Equals(providerID)),
		db.Championship.Category.Set(championshipCategoryF1),
		db.Championship.IsActive.Set(true),
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("create championship: %w", err)
	}

	return championship, nil
}

// ensureEvent returns the provider event row matching the fetched meeting.
func (s *RaceArchiveStore) ensureEvent(ctx context.Context, providerID string, archive domain.RaceArchive) (*db.EventModel, error) {
	eventExternalKey := optionalString(strconv.Itoa(archive.Metadata.MeetingKey))
	if eventExternalKey != nil {
		event, err := s.client.Event.FindFirst(
			db.Event.ProviderID.Equals(providerID),
			db.Event.ExternalKey.Equals(*eventExternalKey),
		).Exec(ctx)
		if err == nil {
			return event, nil
		}
		if !db.IsErrNotFound(err) {
			return nil, fmt.Errorf("find event by external key: %w", err)
		}
	}

	startAt, endAt := inferEventBounds(archive)

	event, err := s.client.Event.CreateOne(
		db.Event.SeasonYear.Set(archive.Metadata.Year),
		db.Event.Name.Set(fallbackString(archive.Metadata.MeetingName, "Race Weekend")),
		db.Event.Location.Set(fallbackString(archive.Metadata.CountryName, "Unknown")),
		db.Event.StartTimeUtc.Set(startAt),
		db.Event.EndTimeUtc.Set(endAt),
		db.Event.Status.Set(resolveEventStatus(startAt, endAt)),
		db.Event.Provider.Link(db.Provider.ID.Equals(providerID)),
		db.Event.OfficialName.SetIfPresent(readOptionalStringField(archive.Meeting, "meeting_official_name")),
		db.Event.CountryName.SetIfPresent(readOptionalStringField(archive.Meeting, "country_name")),
		db.Event.CountryCode.SetIfPresent(readOptionalStringField(archive.Meeting, "country_code")),
		db.Event.CircuitName.SetIfPresent(readOptionalStringField(archive.Meeting, "circuit_short_name")),
		db.Event.ExternalKey.SetIfPresent(eventExternalKey),
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	return event, nil
}

// ensureRace returns the championship race row matching the fetched provider meeting.
func (s *RaceArchiveStore) ensureRace(
	ctx context.Context,
	championshipID string,
	eventID string,
	archive domain.RaceArchive,
) (*db.RaceModel, error) {
	race, err := s.client.Race.FindFirst(
		db.Race.EventID.Equals(eventID),
	).Exec(ctx)
	if err == nil {
		return race, nil
	}
	if !db.IsErrNotFound(err) {
		return nil, fmt.Errorf("find race: %w", err)
	}

	startAt, endAt := inferEventBounds(archive)
	raceParams := []db.RaceSetParam{
		db.Race.RoundNumber.SetIfPresent(readOptionalIntField(archive.Meeting, "meeting_number")),
		db.Race.OfficialName.SetIfPresent(readOptionalStringField(archive.Meeting, "meeting_official_name")),
		db.Race.CountryName.SetIfPresent(readOptionalStringField(archive.Meeting, "country_name")),
		db.Race.CountryCode.SetIfPresent(readOptionalStringField(archive.Meeting, "country_code")),
		db.Race.CircuitName.SetIfPresent(readOptionalStringField(archive.Meeting, "circuit_short_name")),
		db.Race.ExternalKey.SetIfPresent(optionalString(strconv.Itoa(archive.Metadata.MeetingKey))),
		db.Race.EndsAtUtc.SetIfPresent(optionalTime(endAt)),
	}

	race, err = s.client.Race.CreateOne(
		db.Race.SeasonYear.Set(archive.Metadata.Year),
		db.Race.Name.Set(fallbackString(archive.Metadata.MeetingName, "Race Weekend")),
		db.Race.StartsAtUtc.Set(startAt),
		db.Race.Status.Set(resolveEventStatus(startAt, endAt)),
		db.Race.Championship.Link(db.Championship.ID.Equals(championshipID)),
		db.Race.Event.Link(db.Event.ID.Equals(eventID)),
		raceParams...,
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("create race: %w", err)
	}

	return race, nil
}

// ensureSession returns the event session row matching the fetched race session.
func (s *RaceArchiveStore) ensureSession(ctx context.Context, eventID, raceID string, archive domain.RaceArchive) (*db.SessionModel, error) {
	sessionExternalKey := optionalString(strconv.Itoa(archive.Metadata.RaceSessKey))
	if sessionExternalKey != nil {
		session, err := s.client.Session.FindFirst(
			db.Session.EventID.Equals(eventID),
			db.Session.ExternalKey.Equals(*sessionExternalKey),
		).Exec(ctx)
		if err == nil {
			return session, nil
		}
		if !db.IsErrNotFound(err) {
			return nil, fmt.Errorf("find session by external key: %w", err)
		}
	}

	startAt := firstTimeFromRows(archive.RaceSession, "date_start")
	if startAt.IsZero() {
		startAt = archive.Metadata.GeneratedAt
	}
	if startAt.IsZero() {
		startAt = time.Now().UTC()
	}

	endAt := firstTimeFromRows(archive.RaceSession, "date_end")

	params := []db.SessionSetParam{
		db.Session.Name.SetIfPresent(optionalString(archive.Metadata.RaceSession)),
		db.Session.ExternalKey.SetIfPresent(sessionExternalKey),
		db.Session.EndedAtUtc.SetIfPresent(optionalTime(endAt)),
		db.Session.Race.Link(db.Race.ID.Equals(raceID)),
	}

	session, err := s.client.Session.CreateOne(
		db.Session.Type.Set(resolveSessionType(archive.Metadata.RaceSession)),
		db.Session.Status.Set(resolveSessionStatus(startAt, endAt)),
		db.Session.StartedAtUtc.Set(startAt),
		db.Session.Event.Link(db.Event.ID.Equals(eventID)),
		params...,
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

// storeDatasets persists every dataset as one JSON chunk linked to a race archive row.
func (s *RaceArchiveStore) storeDatasets(ctx context.Context, archiveID string, datasets map[string][]map[string]any) error {
	keys := make([]string, 0, len(datasets))
	for key := range datasets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		rows := nonNilRows(datasets[key])
		rowsJSON, err := marshalPrismaJSON(rows)
		if err != nil {
			return fmt.Errorf("marshal dataset %q: %w", key, err)
		}

		if _, err := s.client.RaceDatasetChunk.CreateOne(
			db.RaceDatasetChunk.Dataset.Set(key),
			db.RaceDatasetChunk.Rows.Set(rowsJSON),
			db.RaceDatasetChunk.Archive.Link(db.RaceArchive.ID.Equals(archiveID)),
			db.RaceDatasetChunk.ChunkIndex.Set(0),
			db.RaceDatasetChunk.RowCount.Set(len(rows)),
		).Exec(ctx); err != nil {
			return fmt.Errorf("create dataset chunk %q: %w", key, err)
		}
	}

	return nil
}

// syncParticipants upserts teams and drivers from the provider drivers dataset.
func (s *RaceArchiveStore) syncParticipants(ctx context.Context, providerID string, rows []map[string]any) error {
	for _, row := range rows {
		driverNumber := readIntField(row, "driver_number")
		if driverNumber <= 0 {
			continue
		}

		team, err := s.ensureTeam(ctx, providerID, row)
		if err != nil {
			return fmt.Errorf("ensure team for driver_number=%d: %w", driverNumber, err)
		}

		if err := s.ensureDriver(ctx, providerID, team.ID, row); err != nil {
			return fmt.Errorf("ensure driver_number=%d: %w", driverNumber, err)
		}
	}

	return nil
}

// ensureTeam upserts a provider team using the OpenF1 driver payload.
func (s *RaceArchiveStore) ensureTeam(ctx context.Context, providerID string, row map[string]any) (*db.TeamModel, error) {
	teamName := fallbackString(readStringField(row, "team_name"), "Unknown")
	externalKey := teamExternalKey(teamName)

	team, err := s.client.Team.FindFirst(
		db.Team.ProviderID.Equals(providerID),
		db.Team.ExternalKey.Equals(externalKey),
	).Exec(ctx)
	if err != nil && !db.IsErrNotFound(err) {
		return nil, fmt.Errorf("find team: %w", err)
	}

	params := []db.TeamSetParam{
		db.Team.Name.Set(teamName),
		db.Team.ExternalKey.Set(externalKey),
		db.Team.Code.SetIfPresent(optionalString(readStringField(row, "team_code"))),
		db.Team.ColorHex.SetIfPresent(optionalString(normalizeColor(readStringField(row, "team_colour")))),
	}

	if err == nil {
		updated, updateErr := s.client.Team.FindUnique(
			db.Team.ID.Equals(team.ID),
		).Update(params...).Exec(ctx)
		if updateErr != nil {
			return nil, fmt.Errorf("update team: %w", updateErr)
		}
		return updated, nil
	}

	created, createErr := s.client.Team.CreateOne(
		db.Team.Name.Set(teamName),
		db.Team.Provider.Link(db.Provider.ID.Equals(providerID)),
		params[1:]...,
	).Exec(ctx)
	if createErr != nil {
		return nil, fmt.Errorf("create team: %w", createErr)
	}
	return created, nil
}

// ensureDriver upserts a provider driver using the OpenF1 driver payload.
func (s *RaceArchiveStore) ensureDriver(ctx context.Context, providerID, teamID string, row map[string]any) error {
	driverNumber := readIntField(row, "driver_number")
	if driverNumber <= 0 {
		return nil
	}

	displayName := fallbackString(
		readFirstStringField(row, "full_name", "broadcast_name", "name"),
		fmt.Sprintf("Driver %d", driverNumber),
	)
	externalKey := strconv.Itoa(driverNumber)
	firstName := optionalString(readStringField(row, "first_name"))
	lastName := optionalString(readStringField(row, "last_name"))
	code := optionalString(readFirstStringField(row, "name_acronym", "driver_code", "broadcast_name"))
	countryCode := optionalString(readStringField(row, "country_code"))

	driver, err := s.client.Driver.FindFirst(
		db.Driver.ProviderID.Equals(providerID),
		db.Driver.ExternalKey.Equals(externalKey),
	).Exec(ctx)
	if err != nil && !db.IsErrNotFound(err) {
		return fmt.Errorf("find driver: %w", err)
	}

	params := []db.DriverSetParam{
		db.Driver.DisplayName.Set(displayName),
		db.Driver.Number.Set(driverNumber),
		db.Driver.ExternalKey.Set(externalKey),
		db.Driver.FirstName.SetIfPresent(firstName),
		db.Driver.LastName.SetIfPresent(lastName),
		db.Driver.Code.SetIfPresent(code),
		db.Driver.CountryCode.SetIfPresent(countryCode),
		db.Driver.Team.Link(db.Team.ID.Equals(teamID)),
	}

	if err == nil {
		_, updateErr := s.client.Driver.FindUnique(
			db.Driver.ID.Equals(driver.ID),
		).Update(params...).Exec(ctx)
		if updateErr != nil {
			return fmt.Errorf("update driver: %w", updateErr)
		}
		return nil
	}

	_, createErr := s.client.Driver.CreateOne(
		db.Driver.DisplayName.Set(displayName),
		db.Driver.Number.Set(driverNumber),
		db.Driver.Provider.Link(db.Provider.ID.Equals(providerID)),
		db.Driver.Team.Link(db.Team.ID.Equals(teamID)),
		db.Driver.ExternalKey.Set(externalKey),
		db.Driver.FirstName.SetIfPresent(firstName),
		db.Driver.LastName.SetIfPresent(lastName),
		db.Driver.Code.SetIfPresent(code),
		db.Driver.CountryCode.SetIfPresent(countryCode),
	).Exec(ctx)
	if createErr != nil {
		return fmt.Errorf("create driver: %w", createErr)
	}

	return nil
}

// storeNormalizedSessionData writes provider datasets into the normalized session tables.
func (s *RaceArchiveStore) storeNormalizedSessionData(ctx context.Context, sessionID string, archive domain.RaceArchive) error {
	driverNumber := archive.Metadata.DriverNumber
	if driverNumber > 0 {
		if err := s.clearDriverSessionData(ctx, sessionID, driverNumber); err != nil {
			return err
		}
		if err := s.clearSharedSessionData(ctx, sessionID, archive.Datasets); err != nil {
			return err
		}
	} else {
		if err := s.clearSessionData(ctx, sessionID); err != nil {
			return err
		}
	}

	writers := []struct {
		name string
		fn   func(context.Context, string, []map[string]any) error
		rows []map[string]any
	}{
		{name: "laps", fn: s.insertRaceLaps, rows: archive.Datasets["laps"]},
		{name: "car_data", fn: s.insertRaceTelemetry, rows: archive.Datasets["car_data"]},
		{name: "location", fn: s.insertRaceLocations, rows: archive.Datasets["location"]},
		{name: "position", fn: s.insertRacePositions, rows: archive.Datasets["position"]},
		{name: "intervals", fn: s.insertRaceIntervals, rows: archive.Datasets["intervals"]},
		{name: "stints", fn: s.insertRaceStints, rows: archive.Datasets["stints"]},
		{name: "pit", fn: s.insertRacePitStops, rows: archive.Datasets["pit"]},
		{name: "race_control", fn: s.insertRaceControlEvents, rows: archive.Datasets["race_control"]},
		{name: "team_radio", fn: s.insertTeamRadioMessages, rows: archive.Datasets["team_radio"]},
		{name: "weather", fn: s.insertRaceWeatherSamples, rows: archive.Datasets["weather"]},
		{name: "session_result", fn: s.insertSessionResults, rows: archive.Datasets["session_result"]},
		{name: "starting_grid", fn: s.insertStartingGridRows, rows: archive.Datasets["starting_grid"]},
		{name: "overtakes", fn: s.insertOvertakeEvents, rows: archive.Datasets["overtakes"]},
		{name: "championship_drivers", fn: s.insertDriverChampionshipRows, rows: archive.Datasets["championship_drivers"]},
		{name: "championship_teams", fn: s.insertTeamChampionshipRows, rows: archive.Datasets["championship_teams"]},
	}

	for _, writer := range writers {
		if err := writer.fn(ctx, sessionID, writer.rows); err != nil {
			return fmt.Errorf("store dataset %q: %w", writer.name, err)
		}
	}

	return nil
}

// clearSessionData removes previous normalized rows for a session before reimport.
func (s *RaceArchiveStore) clearSessionData(ctx context.Context, sessionID string) error {
	tables := []string{
		"RaceHighlight",
		"RaceLap",
		"RaceTelemetrySample",
		"RaceLocationSample",
		"RacePositionSample",
		"RaceIntervalSample",
		"RaceStint",
		"RacePitStop",
		"RaceControlEvent",
		"TeamRadioMessage",
		"RaceWeatherSample",
		"SessionResultRow",
		"StartingGridRow",
		"OvertakeEvent",
		"DriverChampionshipStanding",
		"TeamChampionshipStanding",
	}

	for _, table := range tables {
		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1`, table)
		if _, err := s.client.Prisma.Raw.ExecuteRaw(query, sessionID).Exec(ctx); err != nil {
			return fmt.Errorf("clear table %s: %w", table, err)
		}
	}

	return nil
}

// clearDriverSessionData removes only driver-scoped rows for one driver before reimport.
func (s *RaceArchiveStore) clearDriverSessionData(ctx context.Context, sessionID string, driverNumber int) error {
	tables := []string{
		"RaceLap",
		"RaceTelemetrySample",
		"RaceLocationSample",
		"RacePositionSample",
		"RaceIntervalSample",
		"RaceStint",
		"RacePitStop",
		"TeamRadioMessage",
		"SessionResultRow",
		"StartingGridRow",
		"DriverChampionshipStanding",
	}

	for _, table := range tables {
		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1 AND "driverNumber" = $2`, table)
		if _, err := s.client.Prisma.Raw.ExecuteRaw(query, sessionID, driverNumber).Exec(ctx); err != nil {
			return fmt.Errorf("clear driver table %s: %w", table, err)
		}
	}

	return nil
}

// clearSharedSessionData refreshes shared datasets only when the current import includes them.
func (s *RaceArchiveStore) clearSharedSessionData(ctx context.Context, sessionID string, datasets map[string][]map[string]any) error {
	tablesByDataset := map[string]string{
		"race_control":       "RaceControlEvent",
		"weather":            "RaceWeatherSample",
		"overtakes":          "OvertakeEvent",
		"championship_teams": "TeamChampionshipStanding",
	}

	for dataset, table := range tablesByDataset {
		if len(datasets[dataset]) == 0 {
			continue
		}

		query := fmt.Sprintf(`DELETE FROM "%s" WHERE "sessionId" = $1`, table)
		if _, err := s.client.Prisma.Raw.ExecuteRaw(query, sessionID).Exec(ctx); err != nil {
			return fmt.Errorf("clear shared table %s: %w", table, err)
		}
	}

	return nil
}

// insertRaceLaps stores lap rows in the normalized race lap table.
func (s *RaceArchiveStore) insertRaceLaps(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceLap" (
			"sessionId","driverNumber","lapNumber","dateStartUtc","lapDurationSec",
			"sector1Sec","sector2Sec","sector3Sec","speedI1Kph","speedI2Kph",
			"speedTrapKph","isPitOutLap","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'date_start','')::timestamptz,
			NULLIF(row->>'lap_duration','')::double precision,
			NULLIF(row->>'duration_sector_1','')::double precision,
			NULLIF(row->>'duration_sector_2','')::double precision,
			NULLIF(row->>'duration_sector_3','')::double precision,
			NULLIF(row->>'i1_speed','')::double precision::integer,
			NULLIF(row->>'i2_speed','')::double precision::integer,
			NULLIF(row->>'st_speed','')::double precision::integer,
			CASE WHEN row ? 'is_pit_out_lap' THEN (row->>'is_pit_out_lap')::boolean ELSE NULL END,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'lap_number','') IS NOT NULL
	`)
}

// insertRaceTelemetry stores car_data rows in the telemetry table.
func (s *RaceArchiveStore) insertRaceTelemetry(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceTelemetrySample" (
			"sessionId","driverNumber","dateUtc","speedKph","rpm","gear",
			"throttlePct","brakePct","drsState","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'speed','')::double precision::integer,
			NULLIF(row->>'rpm','')::double precision::integer,
			NULLIF(row->>'n_gear','')::double precision::integer,
			NULLIF(row->>'throttle','')::double precision,
			NULLIF(row->>'brake','')::double precision,
			NULLIF(row->>'drs','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertRaceLocations stores location rows in the normalized location table.
func (s *RaceArchiveStore) insertRaceLocations(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceLocationSample" (
			"sessionId","driverNumber","dateUtc","x","y","z","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'x','')::double precision,
			NULLIF(row->>'y','')::double precision,
			NULLIF(row->>'z','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertRacePositions stores in-race positions in the normalized position table.
func (s *RaceArchiveStore) insertRacePositions(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RacePositionSample" (
			"sessionId","driverNumber","dateUtc","lapNumber","position","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'position','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertRaceIntervals stores interval rows in the normalized interval table.
func (s *RaceArchiveStore) insertRaceIntervals(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceIntervalSample" (
			"sessionId","driverNumber","dateUtc","gapToLeader","intervalToFrontDriver","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'gap_to_leader',''),
			COALESCE(NULLIF(row->>'interval_to_front',''), NULLIF(row->>'interval','')),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
		  AND NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertRaceStints stores stint rows in the normalized stint table.
func (s *RaceArchiveStore) insertRaceStints(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceStint" (
			"sessionId","driverNumber","stintNumber","lapStart","lapEnd",
			"compound","tyreAgeLapsStart","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'stint_number','')::double precision::integer,
			NULLIF(row->>'lap_start','')::double precision::integer,
			NULLIF(row->>'lap_end','')::double precision::integer,
			NULLIF(row->>'compound',''),
			COALESCE(
				NULLIF(row->>'tyre_age_at_start','')::double precision::integer,
				NULLIF(row->>'tyre_age_at_start_lap','')::double precision::integer
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertRacePitStops stores pit rows in the normalized pit stop table.
func (s *RaceArchiveStore) insertRacePitStops(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RacePitStop" (
			"sessionId","driverNumber","lapNumber","dateUtc","pitDurationSec","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			COALESCE(
				NULLIF(row->>'pit_duration','')::double precision,
				NULLIF(row->>'pit_duration_sec','')::double precision
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertRaceControlEvents stores race control rows in the normalized race control table.
func (s *RaceArchiveStore) insertRaceControlEvents(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceControlEvent" (
			"sessionId","dateUtc","lapNumber","driverNumber","category","flag","scope","message","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'category',''),
			NULLIF(row->>'flag',''),
			NULLIF(row->>'scope',''),
			NULLIF(row->>'message',''),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertTeamRadioMessages stores radio rows in the normalized team radio table.
func (s *RaceArchiveStore) insertTeamRadioMessages(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "TeamRadioMessage" (
			"sessionId","driverNumber","dateUtc","recordingUrl","transcript","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'recording_url',''),
			NULLIF(row->>'transcript',''),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertRaceWeatherSamples stores weather rows in the normalized weather table.
func (s *RaceArchiveStore) insertRaceWeatherSamples(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "RaceWeatherSample" (
			"sessionId","dateUtc","airTempC","trackTempC","humidityPct","pressureHpa",
			"windSpeedKph","windDirectionDeg","rainfallMm","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'air_temperature','')::double precision,
			NULLIF(row->>'track_temperature','')::double precision,
			NULLIF(row->>'humidity','')::double precision,
			NULLIF(row->>'pressure','')::double precision,
			NULLIF(row->>'wind_speed','')::double precision,
			NULLIF(row->>'wind_direction','')::double precision::integer,
			NULLIF(row->>'rainfall','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'date','') IS NOT NULL
	`)
}

// insertSessionResults stores session result rows in the normalized results table.
func (s *RaceArchiveStore) insertSessionResults(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "SessionResultRow" (
			"sessionId","driverNumber","position","points","status","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'position','')::double precision::integer,
			NULLIF(row->>'points','')::double precision,
			COALESCE(
				NULLIF(row->>'status',''),
				NULLIF(row->>'classification_status',''),
				CASE WHEN lower(COALESCE(row->>'dnf','false')) = 'true' THEN 'DNF' ELSE NULL END
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertStartingGridRows stores starting grid rows in the normalized starting grid table.
func (s *RaceArchiveStore) insertStartingGridRows(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "StartingGridRow" (
			"sessionId","driverNumber","gridPosition","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			COALESCE(
				NULLIF(row->>'grid_position','')::double precision::integer,
				NULLIF(row->>'position','')::double precision::integer
			),
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertOvertakeEvents stores overtakes in the normalized overtake table.
func (s *RaceArchiveStore) insertOvertakeEvents(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "OvertakeEvent" (
			"sessionId","dateUtc","lapNumber","overtakingDriverNumber","overtakenDriverNumber","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'date','')::timestamptz,
			NULLIF(row->>'lap_number','')::double precision::integer,
			COALESCE(
				NULLIF(row->>'overtaking_driver_number','')::double precision::integer,
				NULLIF(row->>'driver_number','')::double precision::integer
			),
			NULLIF(row->>'overtaken_driver_number','')::double precision::integer,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
	`)
}

// insertDriverChampionshipRows stores driver championship standings in the normalized table.
func (s *RaceArchiveStore) insertDriverChampionshipRows(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "DriverChampionshipStanding" (
			"sessionId","driverNumber","positionCurrent","positionStart","pointsCurrent","pointsStart","raw"
		)
		SELECT
			$1,
			NULLIF(row->>'driver_number','')::double precision::integer,
			NULLIF(row->>'position_current','')::double precision::integer,
			NULLIF(row->>'position_start','')::double precision::integer,
			NULLIF(row->>'points_current','')::double precision,
			NULLIF(row->>'points_start','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
		WHERE NULLIF(row->>'driver_number','') IS NOT NULL
	`)
}

// insertTeamChampionshipRows stores constructor championship standings in the normalized table.
func (s *RaceArchiveStore) insertTeamChampionshipRows(ctx context.Context, sessionID string, rows []map[string]any) error {
	return s.executeDatasetInsert(ctx, sessionID, rows, `
		INSERT INTO "TeamChampionshipStanding" (
			"sessionId","teamName","teamCode","positionCurrent","positionStart","pointsCurrent","pointsStart","raw"
		)
		SELECT
			$1,
			COALESCE(NULLIF(row->>'team_name',''), NULLIF(row->>'constructor_name','')),
			COALESCE(NULLIF(row->>'team_code',''), NULLIF(row->>'name_acronym','')),
			NULLIF(row->>'position_current','')::double precision::integer,
			NULLIF(row->>'position_start','')::double precision::integer,
			NULLIF(row->>'points_current','')::double precision,
			NULLIF(row->>'points_start','')::double precision,
			row
		FROM jsonb_array_elements($2::jsonb) AS row
	`)
}

// executeDatasetInsert runs a bulk insert SQL statement from a dataset JSON payload.
func (s *RaceArchiveStore) executeDatasetInsert(ctx context.Context, sessionID string, rows []map[string]any, query string) error {
	payload, err := json.Marshal(nonNilRows(rows))
	if err != nil {
		return fmt.Errorf("marshal dataset rows: %w", err)
	}

	if _, err := s.client.Prisma.Raw.ExecuteRaw(query, sessionID, string(payload)).Exec(ctx); err != nil {
		return err
	}

	return nil
}

// inferEventBounds computes weekend bounds from all known sessions and race fallback timestamps.
func inferEventBounds(archive domain.RaceArchive) (time.Time, time.Time) {
	var start time.Time
	var end time.Time

	for _, row := range archive.AllSessions {
		sessionStart := firstTimeFromRows(row, "date_start")
		sessionEnd := firstTimeFromRows(row, "date_end")

		if !sessionStart.IsZero() && (start.IsZero() || sessionStart.Before(start)) {
			start = sessionStart
		}
		if !sessionEnd.IsZero() && sessionEnd.After(end) {
			end = sessionEnd
		}
	}

	if start.IsZero() {
		start = firstTimeFromRows(archive.RaceSession, "date_start")
	}
	if end.IsZero() {
		end = firstTimeFromRows(archive.RaceSession, "date_end")
	}

	if start.IsZero() {
		start = time.Now().UTC()
	}
	if end.IsZero() {
		end = start.Add(2 * time.Hour)
	}
	if end.Before(start) {
		end = start
	}

	return start, end
}

// resolveArchiveMode maps metadata scope to the persisted archive mode enum.
func resolveArchiveMode(driverNumber int) db.ArchiveMode {
	if driverNumber > 0 {
		return db.ArchiveModeDriverFocused
	}
	return db.ArchiveModeFull
}

// resolveEventStatus infers event status from start/end bounds and current time.
func resolveEventStatus(start, end time.Time) db.EventStatus {
	now := time.Now().UTC()
	if !end.IsZero() && now.After(end) {
		return db.EventStatusFinished
	}
	if !start.IsZero() && now.After(start) {
		return db.EventStatusLive
	}
	return db.EventStatusScheduled
}

// resolveSessionType maps provider session labels to internal enum values.
func resolveSessionType(sessionName string) db.SessionType {
	name := strings.ToLower(strings.TrimSpace(sessionName))
	switch {
	case strings.Contains(name, "sprint"):
		return db.SessionTypeSprint
	case strings.Contains(name, "qual"):
		return db.SessionTypeQuali
	case strings.Contains(name, "practice"), strings.Contains(name, "fp"):
		return db.SessionTypePractice
	default:
		return db.SessionTypeRace
	}
}

// resolveSessionStatus infers session status from start/end bounds and current time.
func resolveSessionStatus(start, end time.Time) db.SessionStatus {
	now := time.Now().UTC()
	if !end.IsZero() && now.After(end) {
		return db.SessionStatusFinished
	}
	if !start.IsZero() && now.After(start) {
		return db.SessionStatusLive
	}
	return db.SessionStatusScheduled
}

// marshalPrismaJSON converts any Go value to prisma JSON payload format.
func marshalPrismaJSON(value any) (db.JSON, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return db.JSON(raw), nil
}

// unmarshalPrismaJSON decodes prisma JSON payloads into Go values.
func unmarshalPrismaJSON(raw db.JSON, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal([]byte(raw), out)
}

// unmarshalMetadataEnvelope decodes metadata in both envelope and legacy metadata-only formats.
func unmarshalMetadataEnvelope(raw db.JSON) (metadataEnvelope, error) {
	out := metadataEnvelope{}
	if err := unmarshalPrismaJSON(raw, &out); err != nil {
		return metadataEnvelope{}, err
	}

	// Legacy fallback when only Metadata was persisted in the JSON field.
	if out.Metadata.Provider == "" && out.Metadata.MeetingKey == 0 && out.Metadata.RaceSessKey == 0 &&
		len(out.AllSessions) == 0 && len(out.Meeting) == 0 && len(out.RaceSession) == 0 {
		legacy := domain.Metadata{}
		if err := unmarshalPrismaJSON(raw, &legacy); err != nil {
			return metadataEnvelope{}, err
		}
		out.Metadata = legacy
	}

	return out, nil
}

// firstTimeFromRows extracts a timestamp field from a provider row map.
func firstTimeFromRows(row map[string]any, key string) time.Time {
	if row == nil {
		return time.Time{}
	}
	raw, ok := row[key]
	if !ok || raw == nil {
		return time.Time{}
	}
	switch v := raw.(type) {
	case time.Time:
		return v.UTC()
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(v))
		if err != nil {
			return time.Time{}
		}
		return parsed.UTC()
	default:
		return time.Time{}
	}
}

// readOptionalStringField reads a non-empty string map value as an optional pointer.
func readOptionalStringField(row map[string]any, key string) *string {
	if row == nil {
		return nil
	}
	raw, ok := row[key]
	if !ok || raw == nil {
		return nil
	}
	s, ok := raw.(string)
	if !ok {
		return nil
	}
	return optionalString(s)
}

// readOptionalIntField reads an optional integer map value as a pointer.
func readOptionalIntField(row map[string]any, key string) *int {
	if row == nil {
		return nil
	}
	raw, ok := row[key]
	if !ok || raw == nil {
		return nil
	}

	switch value := raw.(type) {
	case int:
		return &value
	case int64:
		parsed := int(value)
		return &parsed
	case float64:
		parsed := int(value)
		return &parsed
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil
		}
		return &parsed
	default:
		return nil
	}
}

// fallbackString returns fallback when the input string is blank.
func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// nonNilMap guarantees a non-nil map for JSON serialization and handlers.
func nonNilMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

// nonNilRows guarantees a non-nil rows slice for JSON serialization and handlers.
func nonNilRows(in []map[string]any) []map[string]any {
	if in == nil {
		return []map[string]any{}
	}
	return in
}

// optionalString returns a pointer to non-empty strings.
func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// optionalInt returns a pointer when the value is strictly positive.
func optionalInt(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

// optionalTime returns a pointer when the timestamp is non-zero.
func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

// optionalStringValue unwraps optional strings into plain values for DTOs.
func optionalStringValue(value string) string {
	return strings.TrimSpace(value)
}

// optionalIntPointer copies an optional integer into a pointer for DTOs.
func optionalIntPointer(value int) *int {
	if value == 0 {
		return nil
	}
	copy := value
	return &copy
}

// cloneMap copies a provider row map for safe reuse in merged payloads.
func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

// mergeUniqueRows appends rows once using their JSON representation as a stable dedupe key.
func mergeUniqueRows(existing []map[string]any, incoming []map[string]any, seen map[string]struct{}) []map[string]any {
	if seen == nil {
		seen = map[string]struct{}{}
	}

	for _, row := range incoming {
		key := rowSignature(row)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		existing = append(existing, cloneMap(row))
	}

	return existing
}

// rowSignature produces a deterministic dedupe key for one provider row.
func rowSignature(row map[string]any) string {
	raw, err := json.Marshal(row)
	if err != nil {
		return fmt.Sprint(row)
	}
	return string(raw)
}

// readIntField reads a numeric provider field and returns zero when missing.
func readIntField(row map[string]any, key string) int {
	if row == nil {
		return 0
	}

	raw, ok := row[key]
	if !ok || raw == nil {
		return 0
	}

	switch value := raw.(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
		if parsed, err := value.Float64(); err == nil {
			return int(parsed)
		}
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return 0
		}
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return int(parsed)
		}
	}

	return 0
}

// readStringField reads a provider field and normalizes it to a trimmed string.
func readStringField(row map[string]any, key string) string {
	if row == nil {
		return ""
	}

	raw, ok := row[key]
	if !ok || raw == nil {
		return ""
	}

	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	case json.Number:
		return strings.TrimSpace(value.String())
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case int:
		return strconv.Itoa(value)
	case int8:
		return strconv.FormatInt(int64(value), 10)
	case int16:
		return strconv.FormatInt(int64(value), 10)
	case int32:
		return strconv.FormatInt(int64(value), 10)
	case int64:
		return strconv.FormatInt(value, 10)
	case bool:
		return strconv.FormatBool(value)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

// readFirstStringField returns the first non-empty provider string among several keys.
func readFirstStringField(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := readStringField(row, key); value != "" {
			return value
		}
	}
	return ""
}

// teamExternalKey builds a stable team key from its provider label.
func teamExternalKey(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return "unknown"
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	key := strings.Trim(builder.String(), "-")
	if key == "" {
		return "unknown"
	}
	return key
}

// normalizeColor normalizes provider team colors to #RRGGBB when possible.
func normalizeColor(value string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "#"))
	if trimmed == "" {
		return ""
	}

	if len(trimmed) == 3 {
		trimmed = strings.Repeat(string(trimmed[0]), 2) +
			strings.Repeat(string(trimmed[1]), 2) +
			strings.Repeat(string(trimmed[2]), 2)
	}

	if len(trimmed) != 6 {
		return ""
	}

	for _, r := range trimmed {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return ""
		}
	}

	return "#" + strings.ToUpper(trimmed)
}
