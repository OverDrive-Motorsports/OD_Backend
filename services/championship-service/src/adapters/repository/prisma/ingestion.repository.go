/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.repository.go - Package prismaadapter source file for services/championship-service/src/adapters/repository/prisma.
	##
*/

package prismaadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	db "overdrive/services/championship-service/resources/db"
	contracts "overdrive/shared/contracts/ingestion"
)

type IngestionRepository struct {
	client *db.PrismaClient
	now    func() time.Time
}

// NewIngestionRepository builds and returns a ingestion repository with its required dependencies.
func NewIngestionRepository(client *db.PrismaClient) *IngestionRepository {
	return &IngestionRepository{
		client: client,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// StoreBatch persists the provided ingestion batch into the service database.
func (r *IngestionRepository) StoreBatch(ctx context.Context, batch contracts.Batch) error {
	provider, err := r.ensureProvider(ctx, batch.Provider)
	if err != nil {
		return err
	}

	championship, err := r.ensureChampionship(ctx, provider.ID, batch.Provider, "f1")
	if err != nil {
		return err
	}

	for _, dataset := range batch.Datasets {
		switch dataset.Name {
		case "event_catalog":
			if err := r.storeEvents(ctx, batch.Provider, championship.ID, dataset.Rows); err != nil {
				return fmt.Errorf("store event_catalog: %w", err)
			}
		case "session_catalog":
			if err := r.storeSessions(ctx, batch.Provider, championship.ID, dataset.Rows); err != nil {
				return fmt.Errorf("store session_catalog: %w", err)
			}
		case "driver_catalog":
			if err := r.storeDrivers(ctx, batch.Provider, championship.ID, dataset.Rows); err != nil {
				return fmt.Errorf("store driver_catalog: %w", err)
			}
		case "session_result":
			if err := r.storeSessionResults(ctx, batch.Provider, championship.ID, dataset.Rows); err != nil {
				return fmt.Errorf("store session_result: %w", err)
			}
		case "starting_grid":
			if err := r.storeStartingGrid(ctx, batch.Provider, championship.ID, dataset.Rows); err != nil {
				return fmt.Errorf("store starting_grid: %w", err)
			}
		case "driver_championship_standings":
			if err := r.storeDriverChampionshipStandings(ctx, batch.Provider, championship.ID, dataset.Rows); err != nil {
				return fmt.Errorf("store driver_championship_standings: %w", err)
			}
		case "team_championship_standings":
			if err := r.storeTeamChampionshipStandings(ctx, batch.Provider, championship.ID, dataset.Rows); err != nil {
				return fmt.Errorf("store team_championship_standings: %w", err)
			}
		default:
			return fmt.Errorf("unsupported championship dataset %q", dataset.Name)
		}
	}

	return nil
}

// ensureProvider finds or creates the related database record needed by ingestion.
func (r *IngestionRepository) ensureProvider(ctx context.Context, providerCode string) (*db.ProviderModel, error) {
	code := strings.TrimSpace(providerCode)
	if code == "" {
		code = "openf1"
	}
	name := strings.ToUpper(code[:1]) + code[1:]
	providerID := contracts.ProviderID(code)
	return r.client.Provider.UpsertOne(
		db.Provider.Code.Equals(code),
	).CreateOrUpdate(
		db.Provider.Code.Set(code),
		db.Provider.Name.Set(name),
		db.Provider.Status.Set(db.ProviderStatusActive),
		db.Provider.ID.Set(providerID),
	).Exec(ctx)
}

// ensureChampionship finds or creates the related database record needed by ingestion.
func (r *IngestionRepository) ensureChampionship(ctx context.Context, providerID string, providerCode string, championshipCode string) (*db.ChampionshipModel, error) {
	code := strings.TrimSpace(championshipCode)
	if code == "" {
		code = "f1"
	}

	championshipID := contracts.ChampionshipID(providerCode, code)
	return r.client.Championship.UpsertOne(
		db.Championship.ProviderIDCode(
			db.Championship.ProviderID.Equals(providerID),
			db.Championship.Code.Equals(code),
		),
	).CreateOrUpdate(
		db.Championship.Code.Set(code),
		db.Championship.Name.Set("Formula 1 World Championship"),
		db.Championship.Provider.Link(db.Provider.ID.Equals(providerID)),
		db.Championship.ID.Set(championshipID),
		db.Championship.Category.Set("single-seater"),
		db.Championship.IsActive.Set(true),
	).Exec(ctx)
}

// storeEvents persists mapped events rows into the service database.
func (r *IngestionRepository) storeEvents(ctx context.Context, providerCode string, championshipID string, rows []map[string]any) error {
	for _, row := range rows {
		extKey := stringify(row["meeting_key"])
		if extKey == "" {
			continue
		}
		seasonYear := intValue(row["season_year"])
		name := fallbackString(row["meeting_name"], "Unnamed event")
		location := fallbackString(row["location"], fallbackString(row["country_name"], "unknown"))
		startTime, err := parseTime(row["start_time_utc"])
		if err != nil {
			return err
		}
		endTime, err := parseTime(row["end_time_utc"])
		if err != nil {
			return err
		}

		eventID := contracts.EventID(providerCode, intValue(row["meeting_key"]))

		_, err = r.client.Event.UpsertOne(
			db.Event.ChampionshipIDExternalKey(
				db.Event.ChampionshipID.Equals(championshipID),
				db.Event.ExternalKey.Equals(extKey),
			),
		).CreateOrUpdate(
			db.Event.SeasonYear.Set(seasonYear),
			db.Event.Name.Set(name),
			db.Event.Location.Set(location),
			db.Event.StartTimeUtc.Set(startTime),
			db.Event.EndTimeUtc.Set(endTime),
			db.Event.Status.Set(inferEventStatus(startTime, endTime, r.now())),
			db.Event.Championship.Link(db.Championship.ID.Equals(championshipID)),
			db.Event.ID.Set(eventID),
			db.Event.OfficialName.SetIfPresent(stringPtr(row["meeting_official_name"])),
			db.Event.CountryName.SetIfPresent(stringPtr(row["country_name"])),
			db.Event.CountryCode.SetIfPresent(stringPtr(row["country_code"])),
			db.Event.CircuitName.SetIfPresent(stringPtr(row["circuit_name"])),
			db.Event.ExternalKey.Set(extKey),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// storeSessions persists mapped sessions rows into the service database.
func (r *IngestionRepository) storeSessions(ctx context.Context, providerCode string, championshipID string, rows []map[string]any) error {
	for _, row := range rows {
		event, err := r.ensureEventForSession(ctx, providerCode, championshipID, row)
		if err != nil {
			return err
		}

		extKey := stringify(row["session_key"])
		if extKey == "" {
			continue
		}
		startedAt, err := parseTime(row["start_time_utc"])
		if err != nil {
			return err
		}
		endedAt, err := parseTime(row["end_time_utc"])
		if err != nil {
			return err
		}
		sessionType := mapSessionType(stringify(row["session_type"]))

		sessionID := contracts.SessionID(providerCode, intValue(row["session_key"]))

		_, err = r.client.Session.UpsertOne(
			db.Session.EventIDExternalKey(
				db.Session.EventID.Equals(event.ID),
				db.Session.ExternalKey.Equals(extKey),
			),
		).CreateOrUpdate(
			db.Session.Type.Set(sessionType),
			db.Session.Status.Set(inferSessionStatus(startedAt, endedAt, r.now())),
			db.Session.StartedAtUtc.Set(startedAt),
			db.Session.Event.Link(db.Event.ID.Equals(event.ID)),
			db.Session.ID.Set(sessionID),
			db.Session.Name.SetIfPresent(stringPtr(row["session_name"])),
			db.Session.ExternalKey.Set(extKey),
			db.Session.BroadcastURL.Set(sessionBroadcastURL(extKey)),
			db.Session.EndedAtUtc.SetIfPresent(timePtr(endedAt)),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// ensureEventForSession finds or creates the related database record needed by ingestion.
func (r *IngestionRepository) ensureEventForSession(ctx context.Context, providerCode string, championshipID string, row map[string]any) (*db.EventModel, error) {
	extKey := stringify(row["meeting_key"])
	if extKey == "" {
		return nil, fmt.Errorf("meeting_key is required to persist sessions")
	}
	seasonYear := intValue(row["season_year"])
	if seasonYear == 0 {
		seasonYear = r.now().Year()
	}
	name := fallbackString(row["meeting_name"], fallbackString(row["session_name"], "Unnamed event"))
	location := fallbackString(row["location"], fallbackString(row["country_name"], "unknown"))
	startTime, err := parseTime(row["start_time_utc"])
	if err != nil {
		return nil, err
	}
	endTime, err := parseTime(row["end_time_utc"])
	if err != nil {
		return nil, err
	}

	eventID := contracts.EventID(providerCode, intValue(row["meeting_key"]))

	return r.client.Event.UpsertOne(
		db.Event.ChampionshipIDExternalKey(
			db.Event.ChampionshipID.Equals(championshipID),
			db.Event.ExternalKey.Equals(extKey),
		),
	).CreateOrUpdate(
		db.Event.SeasonYear.Set(seasonYear),
		db.Event.Name.Set(name),
		db.Event.Location.Set(location),
		db.Event.StartTimeUtc.Set(startTime),
		db.Event.EndTimeUtc.Set(endTime),
		db.Event.Status.Set(inferEventStatus(startTime, endTime, r.now())),
		db.Event.Championship.Link(db.Championship.ID.Equals(championshipID)),
		db.Event.ID.Set(eventID),
		db.Event.CountryName.SetIfPresent(stringPtr(row["country_name"])),
		db.Event.CountryCode.SetIfPresent(stringPtr(row["country_code"])),
		db.Event.CircuitName.SetIfPresent(stringPtr(row["circuit_name"])),
		db.Event.ExternalKey.Set(extKey),
	).Exec(ctx)
}

// storeDrivers persists mapped drivers rows into the service database.
func (r *IngestionRepository) storeDrivers(ctx context.Context, providerCode string, championshipID string, rows []map[string]any) error {
	for _, row := range rows {
		team, err := r.ensureTeam(ctx, providerCode, championshipID, row)
		if err != nil {
			return err
		}

		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		extKey := driverExternalKey(row)
		displayName := fallbackString(row["display_name"], fmt.Sprintf("driver-%d", driverNumber))

		driverID := contracts.DriverID(providerCode, "f1", extKey)

		_, err = r.client.Driver.UpsertOne(
			db.Driver.ChampionshipIDExternalKey(
				db.Driver.ChampionshipID.Equals(championshipID),
				db.Driver.ExternalKey.Equals(extKey),
			),
		).CreateOrUpdate(
			db.Driver.DisplayName.Set(displayName),
			db.Driver.Number.Set(driverNumber),
			db.Driver.Championship.Link(db.Championship.ID.Equals(championshipID)),
			db.Driver.Team.Link(db.Team.ID.Equals(team.ID)),
			db.Driver.ID.Set(driverID),
			db.Driver.FirstName.SetIfPresent(stringPtr(row["first_name"])),
			db.Driver.LastName.SetIfPresent(stringPtr(row["last_name"])),
			db.Driver.Code.SetIfPresent(stringPtr(row["driver_code"])),
			db.Driver.CountryCode.SetIfPresent(stringPtr(row["country_code"])),
			db.Driver.ExternalKey.Set(extKey),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// storeSessionResults persists mapped session results rows into the service database.
func (r *IngestionRepository) storeSessionResults(ctx context.Context, providerCode string, championshipID string, rows []map[string]any) error {
	session, err := r.ensureSessionForDataset(ctx, providerCode, championshipID, rows)
	if err != nil {
		return err
	}
	if _, err := r.client.SessionResultRow.FindMany(db.SessionResultRow.SessionID.Equals(session.ID)).Delete().Exec(ctx); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		raw, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.SessionResultRow.CreateOne(
			db.SessionResultRow.DriverNumber.Set(driverNumber),
			db.SessionResultRow.Raw.Set(raw),
			db.SessionResultRow.Session.Link(db.Session.ID.Equals(session.ID)),
			db.SessionResultRow.Position.SetIfPresent(intPtr(row["position"])),
			db.SessionResultRow.Points.SetIfPresent(floatPtr(row["points"])),
			db.SessionResultRow.Status.SetIfPresent(stringPtr(row["status"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeStartingGrid persists mapped starting grid rows into the service database.
func (r *IngestionRepository) storeStartingGrid(ctx context.Context, providerCode string, championshipID string, rows []map[string]any) error {
	session, err := r.ensureSessionForDataset(ctx, providerCode, championshipID, rows)
	if err != nil {
		return err
	}
	if _, err := r.client.StartingGridRow.FindMany(db.StartingGridRow.SessionID.Equals(session.ID)).Delete().Exec(ctx); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		raw, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.StartingGridRow.CreateOne(
			db.StartingGridRow.DriverNumber.Set(driverNumber),
			db.StartingGridRow.Raw.Set(raw),
			db.StartingGridRow.Session.Link(db.Session.ID.Equals(session.ID)),
			db.StartingGridRow.GridPosition.SetIfPresent(intPtr(row["position"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeDriverChampionshipStandings persists mapped driver championship standings rows into the service database.
func (r *IngestionRepository) storeDriverChampionshipStandings(ctx context.Context, providerCode string, championshipID string, rows []map[string]any) error {
	session, err := r.ensureSessionForDataset(ctx, providerCode, championshipID, rows)
	if err != nil {
		return err
	}
	if _, err := r.client.DriverChampionshipStanding.FindMany(db.DriverChampionshipStanding.SessionID.Equals(session.ID)).Delete().Exec(ctx); err != nil {
		return err
	}
	for _, row := range rows {
		driverNumber := intValue(row["driver_number"])
		if driverNumber == 0 {
			continue
		}
		raw, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.DriverChampionshipStanding.CreateOne(
			db.DriverChampionshipStanding.DriverNumber.Set(driverNumber),
			db.DriverChampionshipStanding.Raw.Set(raw),
			db.DriverChampionshipStanding.Session.Link(db.Session.ID.Equals(session.ID)),
			db.DriverChampionshipStanding.PositionCurrent.SetIfPresent(intPtr(row["position_current"])),
			db.DriverChampionshipStanding.PositionStart.SetIfPresent(intPtr(row["position_start"])),
			db.DriverChampionshipStanding.PointsCurrent.SetIfPresent(floatPtr(row["points_current"])),
			db.DriverChampionshipStanding.PointsStart.SetIfPresent(floatPtr(row["points_start"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeTeamChampionshipStandings persists mapped team championship standings rows into the service database.
func (r *IngestionRepository) storeTeamChampionshipStandings(ctx context.Context, providerCode string, championshipID string, rows []map[string]any) error {
	session, err := r.ensureSessionForDataset(ctx, providerCode, championshipID, rows)
	if err != nil {
		return err
	}
	if _, err := r.client.TeamChampionshipStanding.FindMany(db.TeamChampionshipStanding.SessionID.Equals(session.ID)).Delete().Exec(ctx); err != nil {
		return err
	}
	for _, row := range rows {
		raw, err := jsonValue(row)
		if err != nil {
			return err
		}
		_, err = r.client.TeamChampionshipStanding.CreateOne(
			db.TeamChampionshipStanding.Raw.Set(raw),
			db.TeamChampionshipStanding.Session.Link(db.Session.ID.Equals(session.ID)),
			db.TeamChampionshipStanding.TeamName.SetIfPresent(stringPtr(row["team_name"])),
			db.TeamChampionshipStanding.TeamCode.SetIfPresent(stringPtr(row["team_code"])),
			db.TeamChampionshipStanding.PositionCurrent.SetIfPresent(intPtr(row["position_current"])),
			db.TeamChampionshipStanding.PositionStart.SetIfPresent(intPtr(row["position_start"])),
			db.TeamChampionshipStanding.PointsCurrent.SetIfPresent(floatPtr(row["points_current"])),
			db.TeamChampionshipStanding.PointsStart.SetIfPresent(floatPtr(row["points_start"])),
		).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// ensureSessionForDataset finds or creates the related database record needed by ingestion.
func (r *IngestionRepository) ensureSessionForDataset(ctx context.Context, providerCode string, championshipID string, rows []map[string]any) (*db.SessionModel, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("at least one row is required")
	}
	first := rows[0]
	sessionKey := intValue(first["session_key"])
	if sessionKey == 0 {
		return nil, fmt.Errorf("session_key is required")
	}
	sessionID := contracts.SessionID(providerCode, sessionKey)
	session, err := r.client.Session.FindUnique(db.Session.ID.Equals(sessionID)).Exec(ctx)
	if err == nil && session != nil {
		return session, nil
	}
	return nil, fmt.Errorf("session %d is not known in championship-service; ingest session_catalog first", sessionKey)
}

// ensureTeam finds or creates the related database record needed by ingestion.
func (r *IngestionRepository) ensureTeam(ctx context.Context, providerCode string, championshipID string, row map[string]any) (*db.TeamModel, error) {
	name := fallbackString(row["team_name"], "Unknown team")
	extKey := teamExternalKey(row)
	teamID := contracts.TeamID(providerCode, "f1", extKey)
	return r.client.Team.UpsertOne(
		db.Team.ChampionshipIDExternalKey(
			db.Team.ChampionshipID.Equals(championshipID),
			db.Team.ExternalKey.Equals(extKey),
		),
	).CreateOrUpdate(
		db.Team.Name.Set(name),
		db.Team.Championship.Link(db.Championship.ID.Equals(championshipID)),
		db.Team.ID.Set(teamID),
		db.Team.Code.SetIfPresent(stringPtr(row["team_code"])),
		db.Team.ColorHex.SetIfPresent(colorPtr(row["team_color_hex"])),
		db.Team.ExternalKey.Set(extKey),
	).Exec(ctx)
}

// inferEventStatus derives a status value from provider timing data and the current time.
func inferEventStatus(start time.Time, end time.Time, now time.Time) db.EventStatus {
	if now.Before(start) {
		return db.EventStatusScheduled
	}
	if now.After(end) {
		return db.EventStatusFinished
	}
	return db.EventStatusLive
}

// inferSessionStatus derives a status value from provider timing data and the current time.
func inferSessionStatus(start time.Time, end time.Time, now time.Time) db.SessionStatus {
	if now.Before(start) {
		return db.SessionStatusScheduled
	}
	if now.After(end) {
		return db.SessionStatusFinished
	}
	return db.SessionStatusLive
}

// mapSessionType maps OpenF1 session type rows into an internal ingestion dataset.
func mapSessionType(value string) db.SessionType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "race":
		return db.SessionTypeRace
	case "sprint":
		return db.SessionTypeSprint
	case "quali", "qualifying":
		return db.SessionTypeQuali
	default:
		return db.SessionTypePractice
	}
}

// parseTime parses provider timestamp values into the service DateTime type.
func parseTime(value any) (time.Time, error) {
	text := stringify(value)
	if text == "" {
		return time.Time{}, fmt.Errorf("missing time value")
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", text, err)
	}
	return parsed.UTC(), nil
}

// intValue converts common numeric payload values into an int, returning zero when absent.
func intValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return 0
}

// stringify converts dynamic values into their string representation.
func stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.Itoa(int(v))
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.Itoa(int(v))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

// jsonValue converts mapped payload data into the Prisma JSON type.
func jsonValue(value any) (db.JSON, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return db.JSON(encoded), nil
}

// fallbackString returns the mapped string value or a fallback when it is empty.
func fallbackString(value any, fallback string) string {
	text := stringify(value)
	if text == "" {
		return fallback
	}
	return text
}

// stringPtr converts a dynamic value into an optional string pointer.
func stringPtr(value any) *string {
	text := stringify(value)
	if text == "" {
		return nil
	}
	return &text
}

// intPtr converts a dynamic value into an optional int pointer.
func intPtr(value any) *int {
	v := intValue(value)
	if v == 0 {
		return nil
	}
	return &v
}

// floatPtr converts a dynamic value into an optional float pointer.
func floatPtr(value any) *float64 {
	switch v := value.(type) {
	case float64:
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int32:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

// timePtr converts a Go time value into an optional Prisma DateTime pointer.
func timePtr(value time.Time) *db.DateTime {
	if value.IsZero() {
		return nil
	}
	converted := db.DateTime(value)
	return &converted
}

// colorPtr normalizes a team color value into a hex color pointer.
func colorPtr(value any) *string {
	text := stringify(value)
	if text == "" {
		return nil
	}
	if !strings.HasPrefix(text, "#") {
		text = "#" + text
	}
	return &text
}

// sessionBroadcastURL builds the placeholder broadcast URL exposed for a session.
func sessionBroadcastURL(sessionKey string) string {
	if strings.TrimSpace(sessionKey) == "" {
		return ""
	}
	return "https://www.youtube.com/watch?v=dQw4w9WgXcQ&session=" + strings.TrimSpace(sessionKey)
}

// teamExternalKey derives the stable external key used to upsert a team record.
func teamExternalKey(row map[string]any) string {
	if key := stringify(row["team_external_key"]); key != "" {
		return key
	}
	name := strings.ToLower(strings.TrimSpace(stringify(row["team_name"])))
	name = strings.ReplaceAll(name, " ", "-")
	if name == "" {
		return "unknown-team"
	}
	return name
}

// driverExternalKey derives the stable external key used to upsert a driver record.
func driverExternalKey(row map[string]any) string {
	if key := stringify(row["driver_external_key"]); key != "" {
		return key
	}
	if number := stringify(row["driver_number"]); number != "" {
		return number
	}
	if code := strings.ToLower(strings.TrimSpace(stringify(row["driver_code"]))); code != "" {
		return code
	}
	return "unknown-driver"
}
