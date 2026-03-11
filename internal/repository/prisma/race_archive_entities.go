/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_entities.go - Prisma upsert helpers for provider, championship, event, race, and session metadata.
##
*/

package prisma

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"overdrive/internal/domain"
	"overdrive/resources/db"
)

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
	broadcastURL := sessionBroadcastURL(archive.Metadata.RaceSessKey)

	startAt := firstTimeFromRows(archive.RaceSession, "date_start")
	if startAt.IsZero() {
		startAt = archive.Metadata.GeneratedAt
	}
	if startAt.IsZero() {
		startAt = time.Now().UTC()
	}

	endAt := firstTimeFromRows(archive.RaceSession, "date_end")

	if sessionExternalKey != nil {
		session, err := s.client.Session.FindFirst(
			db.Session.EventID.Equals(eventID),
			db.Session.ExternalKey.Equals(*sessionExternalKey),
		).Exec(ctx)
		if err == nil {
			updated, updateErr := s.client.Session.FindUnique(
				db.Session.ID.Equals(session.ID),
			).Update(
				db.Session.Name.SetIfPresent(optionalString(archive.Metadata.RaceSession)),
				db.Session.Status.Set(resolveSessionStatus(startAt, endAt)),
				db.Session.StartedAtUtc.Set(startAt),
				db.Session.EndedAtUtc.SetIfPresent(optionalTime(endAt)),
				db.Session.BroadcastURL.SetIfPresent(optionalString(broadcastURL)),
				db.Session.Race.Link(db.Race.ID.Equals(raceID)),
			).Exec(ctx)
			if updateErr != nil {
				return nil, fmt.Errorf("update session: %w", updateErr)
			}
			return updated, nil
		}
		if !db.IsErrNotFound(err) {
			return nil, fmt.Errorf("find session by external key: %w", err)
		}
	}

	params := []db.SessionSetParam{
		db.Session.Name.SetIfPresent(optionalString(archive.Metadata.RaceSession)),
		db.Session.ExternalKey.SetIfPresent(sessionExternalKey),
		db.Session.BroadcastURL.SetIfPresent(optionalString(broadcastURL)),
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
