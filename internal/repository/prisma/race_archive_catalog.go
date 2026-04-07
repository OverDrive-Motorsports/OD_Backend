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
	"sort"
	"strconv"
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

type driverSessionContext struct {
	sessionKey string
	meetingKey string
	driverName string
	teamName   string
}

type sessionArchiveSnapshot struct {
	envelope metadataEnvelope
	storedAt time.Time
	counts   map[string]int
}

var catalogDatasetOrder = []string{
	"drivers",
	"laps",
	"car_data",
	"location",
	"position",
	"intervals",
	"stints",
	"pit",
	"team_radio",
	"race_control",
	"weather",
	"session_result",
	"starting_grid",
	"overtakes",
	"championship_drivers",
	"championship_teams",
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

// GetSessionMetadata returns the light session metadata envelope without decoding stored dataset chunks.
func (s *RaceArchiveStore) GetSessionMetadata(ctx context.Context, sessionID string) (domain.SessionMetadataWindow, bool, error) {
	snapshot, found, err := s.loadSessionArchiveSnapshot(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return domain.SessionMetadataWindow{}, false, err
	}
	if !found {
		return domain.SessionMetadataWindow{}, true, nil
	}

	return domain.SessionMetadataWindow{
		Metadata:    metadataMap(snapshot.envelope.Metadata),
		Meeting:     nonNilMap(snapshot.envelope.Meeting),
		AllSessions: nonNilRows(snapshot.envelope.AllSessions),
		RaceSession: nonNilMap(snapshot.envelope.RaceSession),
		StoredAt:    snapshot.storedAt,
	}, true, nil
}

// GetSessionDatasetCatalog returns dataset counts without rebuilding the archive payload.
func (s *RaceArchiveStore) GetSessionDatasetCatalog(ctx context.Context, sessionID string) (domain.SessionDatasetCatalogWindow, bool, error) {
	snapshot, found, err := s.loadSessionArchiveSnapshot(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return domain.SessionDatasetCatalogWindow{}, false, err
	}
	if !found {
		return domain.SessionDatasetCatalogWindow{}, true, nil
	}

	items := make([]map[string]any, 0, len(catalogDatasetOrder))
	for _, dataset := range catalogDatasetOrder {
		count, ok := snapshot.counts[dataset]
		if !ok {
			continue
		}
		items = append(items, map[string]any{
			"dataset": dataset,
			"count":   count,
		})
	}

	return domain.SessionDatasetCatalogWindow{
		SessionID: sessionID,
		Metadata:  metadataMap(snapshot.envelope.Metadata),
		Count:     len(items),
		Data:      items,
	}, true, nil
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

// GetSessionDataset returns one session-scoped dataset directly from normalized storage when supported.
func (s *RaceArchiveStore) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetWindow, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	dataset = strings.TrimSpace(dataset)
	if sessionID == "" || dataset == "" {
		return domain.SessionDatasetWindow{}, false, nil
	}


	exists, err := s.sessionExists(ctx, sessionID)
	if err != nil {
		return domain.SessionDatasetWindow{}, false, err
	}
	if !exists {
		return domain.SessionDatasetWindow{}, false, nil
	}

	archiveMetadata, err := s.loadSessionArchiveMetadata(ctx, sessionID)
	if err != nil {
		return domain.SessionDatasetWindow{}, false, err
	}

	var rows []map[string]any
	switch dataset {
	case "drivers":
		rows, err = s.loadDirectSessionDriverRows(ctx, sessionID)
	case "teams":
		rows, err = s.loadDirectSessionTeamRows(ctx, sessionID)
	case "weather":
		rows, err = s.loadDirectWeatherRows(ctx, sessionID)
	case "race_control":
		rows, err = s.loadDirectRaceControlRows(ctx, sessionID)
	default:
		return domain.SessionDatasetWindow{}, false, nil
	}
	if err != nil {
		return domain.SessionDatasetWindow{}, false, err
	}

	return domain.SessionDatasetWindow{
		Dataset:   dataset,
		SessionID: sessionID,
		Count:     len(rows),
		Data:      rows,
		Metadata:  metadataMap(archiveMetadata),
	}, true, nil
}

// GetSessionDriverDataset returns one driver-scoped dataset directly from normalized storage when supported.
func (s *RaceArchiveStore) GetSessionDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) (domain.DriverDatasetWindow, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	dataset = strings.TrimSpace(dataset)
	if sessionID == "" || driverNumber <= 0 || dataset == "" {
		return domain.DriverDatasetWindow{}, false, nil
	}

	exists, err := s.sessionExists(ctx, sessionID)
	if err != nil {
		return domain.DriverDatasetWindow{}, false, err
	}
	if !exists {
		return domain.DriverDatasetWindow{}, false, nil
	}

	meta, err := s.loadDriverSessionContext(ctx, sessionID, driverNumber)
	if err != nil {
		return domain.DriverDatasetWindow{}, false, err
	}
	archiveMetadata, err := s.loadSessionArchiveMetadata(ctx, sessionID)
	if err != nil {
		return domain.DriverDatasetWindow{}, false, err
	}

	var rows []map[string]any
	switch dataset {
	case "laps":
		rows, err = s.loadDirectLapRows(ctx, sessionID, driverNumber, meta)
	case "position":
		rows, err = s.loadDirectPositionRows(ctx, sessionID, driverNumber, meta)
	case "location":
		rows, err = s.loadDirectLocationRows(ctx, sessionID, driverNumber, meta)
	case "car_data":
		rows, err = s.loadDirectTelemetryRows(ctx, sessionID, driverNumber, meta)
	case "intervals":
		rows, err = s.loadDirectIntervalRows(ctx, sessionID, driverNumber, meta)
	case "stints":
		rows, err = s.loadDirectStintRows(ctx, sessionID, driverNumber, meta)
	case "pit":
		rows, err = s.loadDirectPitRows(ctx, sessionID, driverNumber, meta)
	case "team_radio":
		rows, err = s.loadDirectRadioRows(ctx, sessionID, driverNumber, meta)
	default:
		return domain.DriverDatasetWindow{}, false, nil
	}
	if err != nil {
		return domain.DriverDatasetWindow{}, false, err
	}

	return domain.DriverDatasetWindow{
		Dataset:      dataset,
		SessionID:    sessionID,
		DriverNumber: driverNumber,
		DriverName:   meta.driverName,
		TeamName:     meta.teamName,
		Count:        len(rows),
		Data:         rows,
		Metadata:     metadataMap(archiveMetadata),
	}, true, nil
}

// GetSessionDriverLapLocation returns all normalized location samples for one driver during one lap window.
func (s *RaceArchiveStore) GetSessionDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (domain.DriverLapLocationWindow, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || driverNumber <= 0 || lapNumber <= 0 {
		return domain.DriverLapLocationWindow{}, false, nil
	}

	lap, err := s.client.RaceLap.FindFirst(
		db.RaceLap.SessionID.Equals(sessionID),
		db.RaceLap.DriverNumber.Equals(driverNumber),
		db.RaceLap.LapNumber.Equals(lapNumber),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return domain.DriverLapLocationWindow{}, false, nil
		}
		return domain.DriverLapLocationWindow{}, false, fmt.Errorf("find lap: %w", err)
	}

	start, ok := lap.DateStartUtc()
	if !ok {
		return domain.DriverLapLocationWindow{}, false, fmt.Errorf("lap_number=%d has no dateStartUtc", lapNumber)
	}

	end := time.Time{}
	nextLap, err := s.client.RaceLap.FindFirst(
		db.RaceLap.SessionID.Equals(sessionID),
		db.RaceLap.DriverNumber.Equals(driverNumber),
		db.RaceLap.LapNumber.Equals(lapNumber+1),
	).Exec(ctx)
	if err == nil {
		if nextStart, ok := nextLap.DateStartUtc(); ok {
			end = nextStart
		}
	} else if !db.IsErrNotFound(err) {
		return domain.DriverLapLocationWindow{}, false, fmt.Errorf("find next lap: %w", err)
	}

	if end.IsZero() {
		lapDurationSec, ok := lap.LapDurationSec()
		if !ok || lapDurationSec <= 0 {
			return domain.DriverLapLocationWindow{}, false, fmt.Errorf("lap_number=%d has no lapDurationSec and next lap is unavailable", lapNumber)
		}
		end = start.Add(time.Duration(float64(time.Second) * lapDurationSec))
	}

	samples, err := s.client.RaceLocationSample.FindMany(
		db.RaceLocationSample.SessionID.Equals(sessionID),
		db.RaceLocationSample.DriverNumber.Equals(driverNumber),
		db.RaceLocationSample.DateUtc.Gte(start),
		db.RaceLocationSample.DateUtc.Lt(end),
	).
		OrderBy(db.RaceLocationSample.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return domain.DriverLapLocationWindow{}, false, fmt.Errorf("find location samples: %w", err)
	}

	meta, err := s.loadDriverSessionContext(ctx, sessionID, driverNumber)
	if err != nil {
		return domain.DriverLapLocationWindow{}, false, err
	}
	archiveMetadata, err := s.loadSessionArchiveMetadata(ctx, sessionID)
	if err != nil {
		return domain.DriverLapLocationWindow{}, false, err
	}

	rows := make([]map[string]any, 0, len(samples))
	for _, sample := range samples {
		row := map[string]any{
			"date":          sample.DateUtc,
			"driver_number": sample.DriverNumber,
			"x":             nil,
			"y":             nil,
			"z":             nil,
		}
		if x, ok := sample.X(); ok {
			row["x"] = x
		}
		if y, ok := sample.Y(); ok {
			row["y"] = y
		}
		if z, ok := sample.Z(); ok {
			row["z"] = z
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}

	return domain.DriverLapLocationWindow{
		Dataset:      "location",
		SessionID:    sessionID,
		DriverNumber: driverNumber,
		DriverName:   meta.driverName,
		TeamName:     meta.teamName,
		LapNumber:    lapNumber,
		WindowStart:  start,
		WindowEnd:    end,
		Count:        len(rows),
		Data:         rows,
		Metadata:     metadataMap(archiveMetadata),
	}, true, nil
}

// GetSessionRaceStandings returns one standings snapshot directly from normalized position samples.
func (s *RaceArchiveStore) GetSessionRaceStandings(ctx context.Context, sessionID string, at *time.Time) (domain.RaceStandingsWindow, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return domain.RaceStandingsWindow{}, false, nil
	}

	exists, err := s.sessionExists(ctx, sessionID)
	if err != nil {
		return domain.RaceStandingsWindow{}, false, err
	}
	if !exists {
		return domain.RaceStandingsWindow{}, false, nil
	}

	filters := []db.RacePositionSampleWhereParam{
		db.RacePositionSample.SessionID.Equals(sessionID),
	}
	if at != nil {
		filters = append(filters, db.RacePositionSample.DateUtc.Lte(*at))
	}

	models, err := s.client.RacePositionSample.FindMany(filters...).
		OrderBy(db.RacePositionSample.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return domain.RaceStandingsWindow{}, false, fmt.Errorf("find position samples: %w", err)
	}
	if len(models) == 0 {
		return domain.RaceStandingsWindow{}, true, nil
	}

	driverRows, snapshotAt, err := s.buildDirectStandingsRows(ctx, sessionID, models)
	if err != nil {
		return domain.RaceStandingsWindow{}, false, err
	}

	archiveMetadata, err := s.loadSessionArchiveMetadata(ctx, sessionID)
	if err != nil {
		return domain.RaceStandingsWindow{}, false, err
	}

	window := domain.RaceStandingsWindow{
		Dataset:    "position",
		SessionID:  sessionID,
		SnapshotAt: snapshotAt,
		Count:      len(driverRows),
		Data:       driverRows,
		Metadata:   metadataMap(archiveMetadata),
	}
	if at != nil {
		requested := at.UTC()
		window.RequestedAt = &requested
	}
	return window, true, nil
}

func (s *RaceArchiveStore) loadDriverSessionContext(ctx context.Context, sessionID string, driverNumber int) (driverSessionContext, error) {
	ctxOut := driverSessionContext{}
	session, err := s.client.Session.FindUnique(
		db.Session.ID.Equals(sessionID),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return ctxOut, nil
		}
		return ctxOut, fmt.Errorf("find session metadata: %w", err)
	}
	if externalKey, ok := session.ExternalKey(); ok {
		ctxOut.sessionKey = strings.TrimSpace(externalKey)
	}

	event, err := s.client.Event.FindUnique(
		db.Event.ID.Equals(session.EventID),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return ctxOut, nil
		}
		return ctxOut, fmt.Errorf("find event metadata: %w", err)
	}
	if externalKey, ok := event.ExternalKey(); ok {
		ctxOut.meetingKey = strings.TrimSpace(externalKey)
	}

	driver, err := s.client.Driver.FindFirst(
		db.Driver.ChampionshipID.Equals(event.ChampionshipID),
		db.Driver.Number.Equals(driverNumber),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return ctxOut, nil
		}
		return ctxOut, fmt.Errorf("find driver metadata: %w", err)
	}
	ctxOut.driverName = strings.TrimSpace(driver.DisplayName)

	if teamID := strings.TrimSpace(driver.TeamID); teamID != "" {
		team, teamErr := s.client.Team.FindUnique(
			db.Team.ID.Equals(teamID),
		).Exec(ctx)
		if teamErr != nil {
			if !db.IsErrNotFound(teamErr) {
				return ctxOut, fmt.Errorf("find team metadata: %w", teamErr)
			}
			return ctxOut, nil
		}
		ctxOut.teamName = strings.TrimSpace(team.Name)
	}

	return ctxOut, nil
}

func (s *RaceArchiveStore) loadDirectLapRows(ctx context.Context, sessionID string, driverNumber int, meta driverSessionContext) ([]map[string]any, error) {
	models, err := s.client.RaceLap.FindMany(
		db.RaceLap.SessionID.Equals(sessionID),
		db.RaceLap.DriverNumber.Equals(driverNumber),
	).
		OrderBy(db.RaceLap.LapNumber.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find lap rows: %w", err)
	}

	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		row["lap_number"] = model.LapNumber
		if started, ok := model.DateStartUtc(); ok {
			row["date_start"] = started
		}
		if value, ok := model.LapDurationSec(); ok {
			row["lap_duration"] = value
		}
		if value, ok := model.Sector1Sec(); ok {
			row["duration_sector_1"] = value
		}
		if value, ok := model.Sector2Sec(); ok {
			row["duration_sector_2"] = value
		}
		if value, ok := model.Sector3Sec(); ok {
			row["duration_sector_3"] = value
		}
		if value, ok := model.SpeedI1Kph(); ok {
			row["i1_speed"] = value
		}
		if value, ok := model.SpeedI2Kph(); ok {
			row["i2_speed"] = value
		}
		if value, ok := model.SpeedTrapKph(); ok {
			row["st_speed"] = value
		}
		if value, ok := model.IsPitOutLap(); ok {
			row["is_pit_out_lap"] = value
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectPositionRows(ctx context.Context, sessionID string, driverNumber int, meta driverSessionContext) ([]map[string]any, error) {
	models, err := s.client.RacePositionSample.FindMany(
		db.RacePositionSample.SessionID.Equals(sessionID),
		db.RacePositionSample.DriverNumber.Equals(driverNumber),
	).
		OrderBy(db.RacePositionSample.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find position rows: %w", err)
	}

	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		row["date"] = model.DateUtc
		if value, ok := model.LapNumber(); ok {
			row["lap_number"] = value
		}
		if value, ok := model.Position(); ok {
			row["position"] = value
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectLocationRows(ctx context.Context, sessionID string, driverNumber int, meta driverSessionContext) ([]map[string]any, error) {
	models, err := s.client.RaceLocationSample.FindMany(
		db.RaceLocationSample.SessionID.Equals(sessionID),
		db.RaceLocationSample.DriverNumber.Equals(driverNumber),
	).
		OrderBy(db.RaceLocationSample.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find location rows: %w", err)
	}

	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		row["date"] = model.DateUtc
		if value, ok := model.X(); ok {
			row["x"] = value
		}
		if value, ok := model.Y(); ok {
			row["y"] = value
		}
		if value, ok := model.Z(); ok {
			row["z"] = value
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectTelemetryRows(ctx context.Context, sessionID string, driverNumber int, meta driverSessionContext) ([]map[string]any, error) {
	models, err := s.client.RaceTelemetrySample.FindMany(
		db.RaceTelemetrySample.SessionID.Equals(sessionID),
		db.RaceTelemetrySample.DriverNumber.Equals(driverNumber),
	).
		OrderBy(db.RaceTelemetrySample.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find telemetry rows: %w", err)
	}

	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		row["date"] = model.DateUtc
		if value, ok := model.SpeedKph(); ok {
			row["speed"] = value
		}
		if value, ok := model.Rpm(); ok {
			row["rpm"] = value
		}
		if value, ok := model.Gear(); ok {
			row["n_gear"] = value
		}
		if value, ok := model.ThrottlePct(); ok {
			row["throttle"] = value
		}
		if value, ok := model.BrakePct(); ok {
			row["brake"] = value
		}
		if value, ok := model.DrsState(); ok {
			row["drs"] = value
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectIntervalRows(ctx context.Context, sessionID string, driverNumber int, meta driverSessionContext) ([]map[string]any, error) {
	models, err := s.client.RaceIntervalSample.FindMany(
		db.RaceIntervalSample.SessionID.Equals(sessionID),
		db.RaceIntervalSample.DriverNumber.Equals(driverNumber),
	).
		OrderBy(db.RaceIntervalSample.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find interval rows: %w", err)
	}

	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		row["date"] = model.DateUtc
		if value, ok := model.GapToLeader(); ok {
			row["gap_to_leader"] = value
		}
		if value, ok := model.IntervalToFrontDriver(); ok {
			row["interval_to_front"] = value
			row["interval_to_front_driver"] = value
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectSessionDriverRows(ctx context.Context, sessionID string) ([]map[string]any, error) {
	session, err := s.client.Session.FindUnique(
		db.Session.ID.Equals(sessionID),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find session for drivers: %w", err)
	}
	event, err := s.client.Event.FindUnique(
		db.Event.ID.Equals(session.EventID),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find event for drivers: %w", err)
	}
	drivers, err := s.client.Driver.FindMany(
		db.Driver.ChampionshipID.Equals(event.ChampionshipID),
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find drivers: %w", err)
	}
	teams, err := s.client.Team.FindMany(
		db.Team.ChampionshipID.Equals(event.ChampionshipID),
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find teams for drivers: %w", err)
	}
	teamByID := make(map[string]db.TeamModel, len(teams))
	for _, team := range teams {
		teamByID[team.ID] = team
	}

	rows := make([]map[string]any, 0, len(drivers))
	for _, driver := range drivers {
		rows = append(rows, directDriverProfileRow(driver, teamByID[driver.TeamID]))
	}
	sort.Slice(rows, func(i, j int) bool {
		left, _ := rows[i]["driver_number"].(int)
		right, _ := rows[j]["driver_number"].(int)
		return left < right
	})
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectSessionTeamRows(ctx context.Context, sessionID string) ([]map[string]any, error) {
	session, err := s.client.Session.FindUnique(
		db.Session.ID.Equals(sessionID),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find session for teams: %w", err)
	}
	event, err := s.client.Event.FindUnique(
		db.Event.ID.Equals(session.EventID),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find event for teams: %w", err)
	}
	teams, err := s.client.Team.FindMany(
		db.Team.ChampionshipID.Equals(event.ChampionshipID),
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find teams: %w", err)
	}

	rows := make([]map[string]any, 0, len(teams))
	for _, team := range teams {
		row := map[string]any{
			"name": team.Name,
		}
		if code, ok := team.Code(); ok {
			row["code"] = code
		}
		if color, ok := team.ColorHex(); ok {
			row["colour"] = strings.TrimPrefix(color, "#")
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return fmt.Sprint(rows[i]["name"]) < fmt.Sprint(rows[j]["name"])
	})
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectWeatherRows(ctx context.Context, sessionID string) ([]map[string]any, error) {
	models, err := s.client.RaceWeatherSample.FindMany(
		db.RaceWeatherSample.SessionID.Equals(sessionID),
	).
		OrderBy(db.RaceWeatherSample.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find weather rows: %w", err)
	}
	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["date"] = model.DateUtc
		if value, ok := model.AirTempC(); ok {
			row["air_temperature"] = value
		}
		if value, ok := model.TrackTempC(); ok {
			row["track_temperature"] = value
		}
		if value, ok := model.HumidityPct(); ok {
			row["humidity"] = value
		}
		if value, ok := model.PressureHpa(); ok {
			row["pressure"] = value
		}
		if value, ok := model.WindSpeedKph(); ok {
			row["wind_speed"] = value
		}
		if value, ok := model.WindDirectionDeg(); ok {
			row["wind_direction"] = value
		}
		if value, ok := model.RainfallMm(); ok {
			row["rainfall"] = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectRaceControlRows(ctx context.Context, sessionID string) ([]map[string]any, error) {
	models, err := s.client.RaceControlEvent.FindMany(
		db.RaceControlEvent.SessionID.Equals(sessionID),
	).
		OrderBy(db.RaceControlEvent.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find race control rows: %w", err)
	}
	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["date"] = model.DateUtc
		if value, ok := model.LapNumber(); ok {
			row["lap_number"] = value
		}
		if value, ok := model.DriverNumber(); ok {
			row["driver_number"] = value
		}
		if value, ok := model.Category(); ok {
			row["category"] = value
		}
		if value, ok := model.Flag(); ok {
			row["flag"] = value
		}
		if value, ok := model.Scope(); ok {
			row["scope"] = value
		}
		if value, ok := model.Message(); ok {
			row["message"] = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectStintRows(ctx context.Context, sessionID string, driverNumber int, meta driverSessionContext) ([]map[string]any, error) {
	models, err := s.client.RaceStint.FindMany(
		db.RaceStint.SessionID.Equals(sessionID),
		db.RaceStint.DriverNumber.Equals(driverNumber),
	).
		OrderBy(db.RaceStint.LapStart.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find stint rows: %w", err)
	}

	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		if value, ok := model.StintNumber(); ok {
			row["stint_number"] = value
		}
		if value, ok := model.LapStart(); ok {
			row["lap_start"] = value
		}
		if value, ok := model.LapEnd(); ok {
			row["lap_end"] = value
		}
		if value, ok := model.Compound(); ok {
			row["compound"] = value
		}
		if value, ok := model.TyreAgeLapsStart(); ok {
			row["tyre_age_at_start"] = value
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectPitRows(ctx context.Context, sessionID string, driverNumber int, meta driverSessionContext) ([]map[string]any, error) {
	models, err := s.client.RacePitStop.FindMany(
		db.RacePitStop.SessionID.Equals(sessionID),
		db.RacePitStop.DriverNumber.Equals(driverNumber),
	).
		OrderBy(db.RacePitStop.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find pit rows: %w", err)
	}

	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		if value, ok := model.DateUtc(); ok {
			row["date"] = value
		}
		if value, ok := model.LapNumber(); ok {
			row["lap_number"] = value
		}
		if value, ok := model.PitDurationSec(); ok {
			row["pit_duration"] = value
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *RaceArchiveStore) loadDirectRadioRows(ctx context.Context, sessionID string, driverNumber int, meta driverSessionContext) ([]map[string]any, error) {
	models, err := s.client.TeamRadioMessage.FindMany(
		db.TeamRadioMessage.SessionID.Equals(sessionID),
		db.TeamRadioMessage.DriverNumber.Equals(driverNumber),
	).
		OrderBy(db.TeamRadioMessage.DateUtc.Order(db.SortOrderAsc)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("find radio rows: %w", err)
	}

	rows := make([]map[string]any, 0, len(models))
	for _, model := range models {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		if value, ok := model.DateUtc(); ok {
			row["date"] = value
		}
		if value, ok := model.RecordingURL(); ok {
			row["recording_url"] = value
		}
		if value, ok := model.Transcript(); ok {
			row["transcript"] = value
		}
		enrichDriverDatasetRow(row, meta)
		rows = append(rows, row)
	}
	return rows, nil
}

func decodeOrInitRawRow(raw db.JSON) map[string]any {
	row := map[string]any{}
	if err := unmarshalPrismaJSON(raw, &row); err != nil || row == nil {
		return map[string]any{}
	}
	return row
}

func enrichDriverDatasetRow(row map[string]any, meta driverSessionContext) {
	if row == nil {
		return
	}
	if meta.meetingKey != "" {
		if _, exists := row["meeting_key"]; !exists {
			if parsed, err := strconv.Atoi(meta.meetingKey); err == nil {
				row["meeting_key"] = parsed
			} else {
				row["meeting_key"] = meta.meetingKey
			}
		}
	}
	if meta.sessionKey != "" {
		if _, exists := row["session_key"]; !exists {
			if parsed, err := strconv.Atoi(meta.sessionKey); err == nil {
				row["session_key"] = parsed
			} else {
				row["session_key"] = meta.sessionKey
			}
		}
	}
	if meta.driverName != "" {
		row["driver_name"] = meta.driverName
	}
	if meta.teamName != "" {
		row["team_name"] = meta.teamName
	}
}

func (s *RaceArchiveStore) loadSessionArchiveMetadata(ctx context.Context, sessionID string) (domain.Metadata, error) {
	snapshot, found, err := s.loadSessionArchiveSnapshot(ctx, sessionID)
	if err != nil {
		return domain.Metadata{}, err
	}
	if !found {
		return domain.Metadata{}, nil
	}
	return snapshot.envelope.Metadata, nil
}

func metadataMap(meta domain.Metadata) map[string]any {
	if meta == (domain.Metadata{}) {
		return nil
	}
	return map[string]any{
		"generated_at":      meta.GeneratedAt,
		"provider":          meta.Provider,
		"meeting_name":      meta.MeetingName,
		"country_name":      meta.CountryName,
		"year":              meta.Year,
		"meeting_key":       meta.MeetingKey,
		"race_session_name": meta.RaceSession,
		"race_session_key":  meta.RaceSessKey,
		"endpoint_count":    meta.EndpointSize,
		"driver_number":     meta.DriverNumber,
	}
}

func (s *RaceArchiveStore) sessionExists(ctx context.Context, sessionID string) (bool, error) {
	_, err := s.client.Session.FindUnique(
		db.Session.ID.Equals(sessionID),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("find session: %w", err)
	}
	return true, nil
}

func (s *RaceArchiveStore) loadSessionArchiveSnapshot(ctx context.Context, sessionID string) (sessionArchiveSnapshot, bool, error) {
	if sessionID == "" {
		return sessionArchiveSnapshot{}, false, nil
	}
	latest, err := s.client.RaceArchive.FindFirst(
		db.RaceArchive.SessionID.Equals(sessionID),
	).
		OrderBy(db.RaceArchive.GeneratedAt.Order(db.SortOrderDesc)).
		Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return sessionArchiveSnapshot{}, false, nil
		}
		return sessionArchiveSnapshot{}, false, fmt.Errorf("find latest session archive metadata: %w", err)
	}
	envelope, err := unmarshalMetadataEnvelope(latest.Metadata)
	if err != nil {
		return sessionArchiveSnapshot{}, false, fmt.Errorf("decode session archive metadata: %w", err)
	}
	if envelope.Metadata.GeneratedAt.IsZero() {
		envelope.Metadata.GeneratedAt = latest.GeneratedAt
	}
	counts := map[string]int{}
	if err := unmarshalPrismaJSON(latest.Counts, &counts); err != nil {
		return sessionArchiveSnapshot{}, false, fmt.Errorf("decode session archive counts: %w", err)
	}
	return sessionArchiveSnapshot{
		envelope: envelope,
		storedAt: latest.GeneratedAt,
		counts:   counts,
	}, true, nil
}

func (s *RaceArchiveStore) buildDirectStandingsRows(ctx context.Context, sessionID string, models []db.RacePositionSampleModel) ([]map[string]any, time.Time, error) {
	session, err := s.client.Session.FindUnique(
		db.Session.ID.Equals(sessionID),
	).Exec(ctx)
	if err != nil {
		if !db.IsErrNotFound(err) {
			return nil, time.Time{}, fmt.Errorf("find standings session metadata: %w", err)
		}
		return nil, time.Time{}, nil
	}
	event, err := s.client.Event.FindUnique(
		db.Event.ID.Equals(session.EventID),
	).Exec(ctx)
	if err != nil {
		if !db.IsErrNotFound(err) {
			return nil, time.Time{}, fmt.Errorf("find standings event metadata: %w", err)
		}
		return nil, time.Time{}, nil
	}
	drivers, err := s.client.Driver.FindMany(
		db.Driver.ChampionshipID.Equals(event.ChampionshipID),
	).Exec(ctx)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("find standings drivers: %w", err)
	}
	teams, err := s.client.Team.FindMany(
		db.Team.ChampionshipID.Equals(event.ChampionshipID),
	).Exec(ctx)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("find standings teams: %w", err)
	}

	teamByID := make(map[string]db.TeamModel, len(teams))
	for _, team := range teams {
		teamByID[team.ID] = team
	}
	driverByNumber := make(map[int]db.DriverModel, len(drivers))
	for _, driver := range drivers {
		driverByNumber[driver.Number] = driver
	}

	latestByDriver := make(map[int]db.RacePositionSampleModel, len(models))
	snapshotAt := time.Time{}
	for _, model := range models {
		position, ok := model.Position()
		if !ok || position <= 0 {
			continue
		}
		current, found := latestByDriver[model.DriverNumber]
		if !found || model.DateUtc.After(current.DateUtc) {
			latestByDriver[model.DriverNumber] = model
		}
		if model.DateUtc.After(snapshotAt) {
			snapshotAt = model.DateUtc
		}
	}

	rows := make([]map[string]any, 0, len(latestByDriver))
	for driverNumber, model := range latestByDriver {
		row := decodeOrInitRawRow(model.Raw)
		row["driver_number"] = driverNumber
		row["date"] = model.DateUtc
		if value, ok := model.LapNumber(); ok {
			row["lap_number"] = value
		}
		if value, ok := model.Position(); ok {
			row["position"] = value
		}
		if driver, ok := driverByNumber[driverNumber]; ok {
			team := teamByID[driver.TeamID]
			row["driver"] = directDriverProfileRow(driver, team)
			row["driver_name"] = driver.DisplayName
			row["team_name"] = team.Name
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		left, _ := rows[i]["position"].(int)
		right, _ := rows[j]["position"].(int)
		return left < right
	})
	return rows, snapshotAt, nil
}

func directDriverProfileRow(driver db.DriverModel, team db.TeamModel) map[string]any {
	row := map[string]any{
		"driver_number":  driver.Number,
		"full_name":      driver.DisplayName,
		"broadcast_name": driver.DisplayName,
		"team_name":      team.Name,
	}
	if firstName, ok := driver.FirstName(); ok {
		row["first_name"] = firstName
	}
	if lastName, ok := driver.LastName(); ok {
		row["last_name"] = lastName
	}
	if code, ok := driver.Code(); ok {
		row["name_acronym"] = code
	}
	if countryCode, ok := driver.CountryCode(); ok {
		row["country_code"] = countryCode
	}
	if color, ok := team.ColorHex(); ok {
		row["team_colour"] = strings.TrimPrefix(color, "#")
	}
	return row
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
