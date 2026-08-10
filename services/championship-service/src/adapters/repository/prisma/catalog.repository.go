/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## catalog.repository.go - Package prismaadapter source file for services/championship-service/src/adapters/repository/prisma.
	##
*/

package prismaadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	db "overdrive/services/championship-service/resources/db"
	"overdrive/services/championship-service/src/core/domain"
)

type CatalogRepository struct {
	client *db.PrismaClient
}

// isNotFoundErr reports whether err is the Prisma "no row matched" sentinel returned
// by FindUnique — this client returns (nil, ErrNotFound) rather than (nil, nil) on a
// miss, so callers must check for it explicitly instead of treating it as a hard error.
func isNotFoundErr(err error) bool {
	return errors.Is(err, db.ErrNotFound)
}

// NewCatalogRepository builds and returns a catalog repository with its required dependencies.
func NewCatalogRepository(client *db.PrismaClient) *CatalogRepository {
	return &CatalogRepository{client: client}
}

// ListChampionships returns a collection of championships for the requested context.
func (r *CatalogRepository) ListChampionships(ctx context.Context) ([]domain.ChampionshipSummary, error) {
	rows, err := r.client.Championship.FindMany().Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.ChampionshipSummary, 0, len(rows))
	for _, row := range rows {
		category, _ := row.Category()
		provider, err := r.client.Provider.FindUnique(db.Provider.ID.Equals(row.ProviderID)).Exec(ctx)
		providerCode := ""
		if err == nil && provider != nil {
			providerCode = provider.Code
		}
		items = append(items, domain.ChampionshipSummary{
			ID:               row.ID,
			ChampionshipCode: row.Code,
			Name:             row.Name,
			Provider:         providerCode,
			Category:         category,
			IsActive:         row.IsActive,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ChampionshipCode < items[j].ChampionshipCode })
	return items, nil
}

// ListEventsByChampionship returns a collection of events by championship for the requested context.
func (r *CatalogRepository) ListEventsByChampionship(ctx context.Context, code string) ([]domain.EventSummary, error) {
	championship, err := r.findChampionshipByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if championship == nil {
		return nil, nil
	}
	rows, err := r.client.Event.FindMany(db.Event.ChampionshipID.Equals(championship.ID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.EventSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapEventSummary(&row, code))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartDate.Before(items[j].StartDate) })
	return items, nil
}

// GetEvent returns the requested event payload for the supplied identifiers.
func (r *CatalogRepository) GetEvent(ctx context.Context, eventID string) (*domain.EventSummary, error) {
	row, err := r.client.Event.FindUnique(db.Event.ID.Equals(eventID)).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	championshipCode := ""
	if championship, cErr := r.client.Championship.FindUnique(db.Championship.ID.Equals(row.ChampionshipID)).Exec(ctx); cErr == nil && championship != nil {
		championshipCode = championship.Code
	}
	item := mapEventSummary(row, championshipCode)
	return &item, nil
}

// ListSessionsByEvent returns a collection of sessions by event for the requested context.
func (r *CatalogRepository) ListSessionsByEvent(ctx context.Context, eventID string) ([]domain.SessionSummary, error) {
	rows, err := r.client.Session.FindMany(db.Session.EventID.Equals(eventID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.SessionSummary, 0, len(rows))
	circuit := ""
	if event, evErr := r.client.Event.FindUnique(db.Event.ID.Equals(eventID)).Exec(ctx); evErr == nil && event != nil {
		if name, ok := event.CircuitName(); ok {
			circuit = name
		}
	}
	for _, row := range rows {
		items = append(items, mapSessionSummary(&row, circuit))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartTime.Before(items[j].StartTime) })
	return items, nil
}

// GetSession returns the requested session payload for the supplied identifiers.
func (r *CatalogRepository) GetSession(ctx context.Context, sessionID string) (*domain.SessionSummary, error) {
	row, err := r.client.Session.FindUnique(db.Session.ID.Equals(sessionID)).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	circuit := ""
	if event, evErr := r.client.Event.FindUnique(db.Event.ID.Equals(row.EventID)).Exec(ctx); evErr == nil && event != nil {
		if name, ok := event.CircuitName(); ok {
			circuit = name
		}
	}
	item := mapSessionSummary(row, circuit)
	return &item, nil
}

// ListSessionDrivers returns a collection of session drivers for the requested context,
// optionally filtered by teamId.
func (r *CatalogRepository) ListSessionDrivers(ctx context.Context, sessionID string, teamID string) ([]domain.DriverSummary, error) {
	session, event, championship, err := r.loadSessionContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	drivers, err := r.client.Driver.FindMany(db.Driver.ChampionshipID.Equals(championship.ID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	teams, err := r.client.Team.FindMany(db.Team.ChampionshipID.Equals(championship.ID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	_ = event
	teamByID := make(map[string]db.TeamModel, len(teams))
	for _, team := range teams {
		teamByID[team.ID] = team
	}
	items := make([]domain.DriverSummary, 0, len(drivers))
	for _, driver := range drivers {
		if teamID != "" && driver.TeamID != teamID {
			continue
		}
		team := teamByID[driver.TeamID]
		firstName, _ := driver.FirstName()
		lastName, _ := driver.LastName()
		code, _ := driver.Code()
		countryCode, _ := driver.CountryCode()
		color, _ := team.ColorHex()
		items = append(items, domain.DriverSummary{
			ID:           driver.ID,
			DriverNumber: driver.Number,
			FullName:     driver.DisplayName,
			FirstName:    firstName,
			LastName:     lastName,
			Code:         code,
			CountryCode:  countryCode,
			TeamID:       team.ID,
			TeamName:     team.Name,
			TeamColor:    color,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DriverNumber < items[j].DriverNumber })
	return items, nil
}

// ListSessionTeams returns a collection of session teams for the requested context.
func (r *CatalogRepository) ListSessionTeams(ctx context.Context, sessionID string) ([]domain.TeamSummary, error) {
	session, _, championship, err := r.loadSessionContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	rows, err := r.client.Team.FindMany(db.Team.ChampionshipID.Equals(championship.ID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.TeamSummary, 0, len(rows))
	for _, row := range rows {
		code, _ := row.Code()
		color, _ := row.ColorHex()
		items = append(items, domain.TeamSummary{
			ID:       row.ID,
			Name:     row.Name,
			Code:     code,
			ColorHex: color,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// GetSessionStandings returns the generic session standings (race result), optionally
// isolating a single driver via driverNumber. It generalizes the former
// GET /sessions/{sessionId}/standings/race endpoint.
func (r *CatalogRepository) GetSessionStandings(ctx context.Context, sessionID string, driverNumber *int) ([]domain.StandingRow, error) {
	session, _, championship, err := r.loadSessionContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	teamLookup, err := r.loadDriverTeamLookup(ctx, championship.ID)
	if err != nil {
		return nil, err
	}
	params := []db.SessionResultRowWhereParam{db.SessionResultRow.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.SessionResultRow.DriverNumber.Equals(*driverNumber))
	}
	rows, err := r.client.SessionResultRow.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.StandingRow, 0, len(rows))
	for _, row := range rows {
		position, _ := row.Position()
		points, _ := row.Points()
		raw := rawMap(row.Raw)
		gapToLeader := stringifyAny(raw["gap_to_leader"])
		items = append(items, domain.StandingRow{
			Position:     position,
			DriverNumber: row.DriverNumber,
			TeamID:       teamLookup[row.DriverNumber],
			GapToLeader:  gapToLeader,
			Points:       points,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	return items, nil
}

// GetDriverProfile returns the global (session-independent) driver profile for the
// supplied driver number, optionally scoped by championshipCode.
func (r *CatalogRepository) GetDriverProfile(ctx context.Context, driverNumber int, championshipCode string) (*domain.DriverProfile, error) {
	params := []db.DriverWhereParam{db.Driver.Number.Equals(driverNumber)}
	if championshipCode != "" {
		championship, err := r.findChampionshipByCode(ctx, championshipCode)
		if err != nil {
			return nil, err
		}
		if championship == nil {
			return nil, nil
		}
		params = append(params, db.Driver.ChampionshipID.Equals(championship.ID))
	}
	driver, err := r.client.Driver.FindFirst(params...).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if driver == nil {
		return nil, nil
	}
	championship, err := r.client.Championship.FindUnique(db.Championship.ID.Equals(driver.ChampionshipID)).Exec(ctx)
	if err != nil && !isNotFoundErr(err) {
		return nil, err
	}
	code := championshipCode
	if championship != nil {
		code = championship.Code
	}
	countryCode, _ := driver.CountryCode()
	return &domain.DriverProfile{
		DriverNumber:     driver.Number,
		FullName:         driver.DisplayName,
		Nationality:      countryCode,
		CurrentTeamID:    driver.TeamID,
		ChampionshipCode: code,
		// driverPicture: no data source available yet (see doc/endpoint.md gap note).
	}, nil
}

// loadDriverTeamLookup builds a driverNumber -> teamId lookup for a championship.
func (r *CatalogRepository) loadDriverTeamLookup(ctx context.Context, championshipID string) (map[int]string, error) {
	drivers, err := r.client.Driver.FindMany(db.Driver.ChampionshipID.Equals(championshipID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	lookup := make(map[int]string, len(drivers))
	for _, driver := range drivers {
		lookup[driver.Number] = driver.TeamID
	}
	return lookup, nil
}

// stringifyAny renders a dynamic raw JSON value as a display string.
func stringifyAny(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

// GetSessionDataset returns the requested session dataset payload for the supplied identifiers.
func (r *CatalogRepository) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (domain.SessionDatasetResponse, error) {
	session, event, championship, err := r.loadSessionContext(ctx, sessionID)
	if err != nil {
		return domain.SessionDatasetResponse{}, err
	}
	if session == nil {
		return domain.SessionDatasetResponse{}, nil
	}
	driverLookup, err := r.loadDriverLookup(ctx, championship.ID)
	if err != nil {
		return domain.SessionDatasetResponse{}, err
	}
	metadata := sessionMetadata(event, session)
	switch dataset {
	case "session_result":
		rows, err := r.client.SessionResultRow.FindMany(db.SessionResultRow.SessionID.Equals(sessionID)).Exec(ctx)
		if err != nil {
			return domain.SessionDatasetResponse{}, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			item := rawMap(row.Raw)
			enrichDriverRow(item, row.DriverNumber, driverLookup)
			data = append(data, item)
		}
		return domain.SessionDatasetResponse{SessionID: sessionID, Dataset: dataset, Count: len(data), Data: data, Metadata: metadata}, nil
	case "starting_grid":
		rows, err := r.client.StartingGridRow.FindMany(db.StartingGridRow.SessionID.Equals(sessionID)).Exec(ctx)
		if err != nil {
			return domain.SessionDatasetResponse{}, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			item := rawMap(row.Raw)
			enrichDriverRow(item, row.DriverNumber, driverLookup)
			data = append(data, item)
		}
		return domain.SessionDatasetResponse{SessionID: sessionID, Dataset: dataset, Count: len(data), Data: data, Metadata: metadata}, nil
	case "championship_drivers":
		rows, err := r.client.DriverChampionshipStanding.FindMany(db.DriverChampionshipStanding.SessionID.Equals(sessionID)).Exec(ctx)
		if err != nil {
			return domain.SessionDatasetResponse{}, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			item := rawMap(row.Raw)
			enrichDriverRow(item, row.DriverNumber, driverLookup)
			data = append(data, item)
		}
		return domain.SessionDatasetResponse{SessionID: sessionID, Dataset: dataset, Count: len(data), Data: data, Metadata: metadata}, nil
	case "championship_teams":
		rows, err := r.client.TeamChampionshipStanding.FindMany(db.TeamChampionshipStanding.SessionID.Equals(sessionID)).Exec(ctx)
		if err != nil {
			return domain.SessionDatasetResponse{}, err
		}
		data := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			data = append(data, rawMap(row.Raw))
		}
		return domain.SessionDatasetResponse{SessionID: sessionID, Dataset: dataset, Count: len(data), Data: data, Metadata: metadata}, nil
	default:
		return domain.SessionDatasetResponse{}, fmt.Errorf("%w %q", domain.ErrUnknownDataset, dataset)
	}
}

type driverLookupRow struct {
	name      string
	teamName  string
	teamColor string
}

// loadDriverLookup implements the load driver lookup workflow for this package.
func (r *CatalogRepository) loadDriverLookup(ctx context.Context, championshipID string) (map[int]driverLookupRow, error) {
	drivers, err := r.client.Driver.FindMany(db.Driver.ChampionshipID.Equals(championshipID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	teams, err := r.client.Team.FindMany(db.Team.ChampionshipID.Equals(championshipID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	teamByID := make(map[string]db.TeamModel, len(teams))
	for _, team := range teams {
		teamByID[team.ID] = team
	}
	lookup := make(map[int]driverLookupRow, len(drivers))
	for _, driver := range drivers {
		team := teamByID[driver.TeamID]
		color, _ := team.ColorHex()
		lookup[driver.Number] = driverLookupRow{
			name:      driver.DisplayName,
			teamName:  team.Name,
			teamColor: color,
		}
	}
	return lookup, nil
}

// enrichDriverRow adds championship driver metadata to a race-data response row.
func enrichDriverRow(item map[string]any, driverNumber int, lookup map[int]driverLookupRow) {
	item["driver_number"] = driverNumber
	if row, ok := lookup[driverNumber]; ok {
		if _, exists := item["driver_name"]; !exists {
			item["driver_name"] = row.name
		}
		if _, exists := item["team_name"]; !exists {
			item["team_name"] = row.teamName
		}
		if row.teamColor != "" {
			item["team_color"] = row.teamColor
		}
	}
}

// rawMap unwraps Prisma JSON into a mutable map response payload.
func rawMap(value db.JSON) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

// loadSessionContext implements the load session context workflow for this package.
func (r *CatalogRepository) loadSessionContext(ctx context.Context, sessionID string) (*db.SessionModel, *db.EventModel, *db.ChampionshipModel, error) {
	session, err := r.client.Session.FindUnique(db.Session.ID.Equals(sessionID)).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, err
	}
	if session == nil {
		return nil, nil, nil, nil
	}
	event, err := r.client.Event.FindUnique(db.Event.ID.Equals(session.EventID)).Exec(ctx)
	if err != nil && !isNotFoundErr(err) {
		return nil, nil, nil, err
	}
	if event == nil {
		return session, nil, nil, fmt.Errorf("event %s not found for session %s", session.EventID, session.ID)
	}
	championship, err := r.client.Championship.FindUnique(db.Championship.ID.Equals(event.ChampionshipID)).Exec(ctx)
	if err != nil && !isNotFoundErr(err) {
		return nil, nil, nil, err
	}
	if championship == nil {
		return session, event, nil, fmt.Errorf("championship %s not found for event %s", event.ChampionshipID, event.ID)
	}
	return session, event, championship, nil
}

// findChampionshipByCode implements the find championship by code workflow for this package.
func (r *CatalogRepository) findChampionshipByCode(ctx context.Context, code string) (*db.ChampionshipModel, error) {
	row, err := r.client.Championship.FindFirst(db.Championship.Code.Equals(code)).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return row, nil
}

// mapEventSummary maps OpenF1 event summary rows into an internal ingestion dataset.
func mapEventSummary(row *db.EventModel, championshipCode string) domain.EventSummary {
	officialName, _ := row.OfficialName()
	countryName, _ := row.CountryName()
	countryCode, _ := row.CountryCode()
	circuitName, _ := row.CircuitName()
	externalKey, _ := row.ExternalKey()
	roundNumber, _ := row.RoundNumber()
	return domain.EventSummary{
		ID:               row.ID,
		ChampionshipID:   row.ChampionshipID,
		ChampionshipCode: championshipCode,
		SeasonYear:       row.SeasonYear,
		RoundNumber:      intPtrFromInt(roundNumber),
		Name:             row.Name,
		OfficialName:     officialName,
		Circuit:          circuitName,
		Location:         row.Location,
		CountryName:      countryName,
		CountryCode:      countryCode,
		ExternalKey:      externalKey,
		Status:           string(row.Status),
		StartDate:        timeValue(row.StartTimeUtc),
		EndDate:          timeValue(row.EndTimeUtc),
	}
}

// mapSessionSummary maps OpenF1 session summary rows into an internal ingestion dataset.
func mapSessionSummary(row *db.SessionModel, circuit string) domain.SessionSummary {
	name, _ := row.Name()
	externalKey, _ := row.ExternalKey()
	broadcastURL, _ := row.BroadcastURL()
	endedAt, ok := row.EndedAtUtc()
	var endedAtPtr *time.Time
	if ok {
		value := time.Time(endedAt)
		endedAtPtr = &value
	}
	return domain.SessionSummary{
		ID:           row.ID,
		EventID:      row.EventID,
		Type:         string(row.Type),
		Status:       string(row.Status),
		Name:         name,
		Circuit:      circuit,
		ExternalKey:  externalKey,
		BroadcastURL: broadcastURL,
		StartTime:    timeValue(row.StartedAtUtc),
		EndTime:      endedAtPtr,
	}
}

// sessionMetadata combines event and session references into API response metadata.
func sessionMetadata(event *db.EventModel, session *db.SessionModel) map[string]any {
	meetingKey, _ := event.ExternalKey()
	sessionKey, _ := session.ExternalKey()
	countryName, _ := event.CountryName()
	sessionName, _ := session.Name()
	return map[string]any{
		"provider":          "openf1",
		"meeting_name":      event.Name,
		"country_name":      countryName,
		"year":              event.SeasonYear,
		"meeting_key":       meetingKey,
		"race_session_name": sessionName,
		"race_session_key":  sessionKey,
	}
}

// intPtrFromInt implements the int ptr from int workflow for this package.
func intPtrFromInt(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

// timeValue implements the time value workflow for this package.
func timeValue(value db.DateTime) time.Time {
	return time.Time(value)
}
