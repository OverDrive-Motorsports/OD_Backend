/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## session_query.usecase.go - Package usecases source file for services/race-data-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"overdrive/services/race-data-service/src/core/ports"
)

type SessionQueryUseCase struct {
	repository ports.SessionQueryRepository
	client     ports.ChampionshipClient
}

// NewSessionQueryUseCase builds and returns a session query use case with its required dependencies.
func NewSessionQueryUseCase(repository ports.SessionQueryRepository, client ports.ChampionshipClient) *SessionQueryUseCase {
	return &SessionQueryUseCase{repository: repository, client: client}
}

// ListChampionships returns a collection of championships for the requested context.
func (u *SessionQueryUseCase) ListChampionships(ctx context.Context) (any, error) {
	return u.client.ListChampionships(ctx)
}

// ListChampionshipEvents returns a collection of championship events for the requested context.
func (u *SessionQueryUseCase) ListChampionshipEvents(ctx context.Context, code string) (any, error) {
	return u.client.ListChampionshipEvents(ctx, code)
}

// GetEvent returns the requested event payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetEvent(ctx context.Context, eventID string) (any, error) {
	return u.client.GetEventPayload(ctx, eventID)
}

// ListEventSessions returns a collection of event sessions for the requested context.
func (u *SessionQueryUseCase) ListEventSessions(ctx context.Context, eventID string) (any, error) {
	return u.client.ListEventSessions(ctx, eventID)
}

// GetSession returns the requested session payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetSession(ctx context.Context, sessionID string) (map[string]any, error) {
	session, err := u.client.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	return map[string]any{
		"id":             session.ID,
		"event_id":       session.EventID,
		"type":           session.Type,
		"status":         session.Status,
		"name":           session.Name,
		"external_key":   session.ExternalKey,
		"broadcast_url":  session.BroadcastURL,
		"started_at_utc": session.StartedAtUTC,
		"ended_at_utc":   session.EndedAtUTC,
	}, nil
}

// GetSessionMetadata returns the requested session metadata payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetSessionMetadata(ctx context.Context, sessionID string) (map[string]any, error) {
	session, event, err := u.loadContext(ctx, sessionID)
	if err != nil || session == nil {
		return nil, err
	}
	return map[string]any{
		"session_id": sessionID,
		"metadata":   sessionMetadata(event, session),
	}, nil
}

// ListSessionDrivers returns a collection of session drivers for the requested context.
func (u *SessionQueryUseCase) ListSessionDrivers(ctx context.Context, sessionID string) (map[string]any, error) {
	drivers, err := u.client.ListSessionDrivers(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if drivers == nil {
		return nil, nil
	}
	data := make([]map[string]any, 0, len(drivers))
	for _, driver := range drivers {
		data = append(data, map[string]any{
			"driver_number": driver.DriverNumber,
			"driver_name":   driver.DriverName,
			"team_name":     driver.TeamName,
			"team_color":    driver.TeamColor,
		})
	}
	return map[string]any{"count": len(data), "data": data, "session_id": sessionID}, nil
}

// ListSessionTeams returns a collection of session teams for the requested context.
func (u *SessionQueryUseCase) ListSessionTeams(ctx context.Context, sessionID string) (map[string]any, error) {
	payload, err := u.client.ListSessionTeams(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, nil
	}
	return map[string]any{"count": len(payload), "data": payload, "session_id": sessionID}, nil
}

// GetSessionDataset returns the requested session dataset payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (map[string]any, error) {
	session, event, err := u.loadContext(ctx, sessionID)
	if err != nil || session == nil {
		return nil, err
	}
	if isRemoteDataset(dataset) {
		return u.client.GetSessionDataset(ctx, sessionID, dataset)
	}
	rows, err := u.repository.GetSessionDataset(ctx, sessionID, dataset)
	if err != nil {
		return nil, err
	}
	lookup, err := u.driverLookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		enrichDriverRow(row, lookup)
	}
	return map[string]any{
		"session_id": sessionID,
		"dataset":    dataset,
		"count":      len(rows),
		"data":       rows,
		"metadata":   sessionMetadata(event, session),
	}, nil
}

// GetSessionRaceStandings returns the requested session race standings payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetSessionRaceStandings(ctx context.Context, sessionID string) (any, error) {
	return u.client.GetSessionRaceStandings(ctx, sessionID)
}

// GetSessionBroadcast returns the requested session broadcast payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetSessionBroadcast(ctx context.Context, sessionID string) (map[string]any, error) {
	return u.client.GetSessionBroadcast(ctx, sessionID)
}

// GetDriverProfile returns the requested driver profile payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetDriverProfile(ctx context.Context, sessionID string, driverNumber int) (map[string]any, error) {
	drivers, err := u.client.ListSessionDrivers(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if drivers == nil {
		return nil, nil
	}
	for _, driver := range drivers {
		if driver.DriverNumber != driverNumber {
			continue
		}
		broadcastURL, err := u.repository.GetDriverBroadcast(ctx, sessionID, driverNumber)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"session_id":    sessionID,
			"driver_number": driver.DriverNumber,
			"driver_name":   driver.DriverName,
			"team_name":     driver.TeamName,
			"team_color":    driver.TeamColor,
			"broadcast_url": broadcastURL,
		}, nil
	}
	return nil, nil
}

// GetDriverBroadcast returns the requested driver broadcast payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (map[string]any, error) {
	session, _, err := u.loadContext(ctx, sessionID)
	if err != nil || session == nil {
		return nil, err
	}
	broadcastURL, err := u.repository.GetDriverBroadcast(ctx, sessionID, driverNumber)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"session_id":    sessionID,
		"driver_number": driverNumber,
		"broadcast_url": broadcastURL,
	}, nil
}

// GetDriverDataset returns the requested driver dataset payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string) (map[string]any, error) {
	session, event, err := u.loadContext(ctx, sessionID)
	if err != nil || session == nil {
		return nil, err
	}
	if dataset == "result" {
		payload, err := u.client.GetSessionDataset(ctx, sessionID, "session_result")
		if err != nil {
			return nil, err
		}
		if payload == nil {
			payload = map[string]any{
				"session_id": sessionID,
				"dataset":    "session_result",
				"metadata":   sessionMetadata(event, session),
			}
		}
		data, _ := payload["data"].([]any)
		filtered := make([]map[string]any, 0, 1)
		for _, item := range data {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if intFromAny(row["driver_number"]) == driverNumber {
				filtered = append(filtered, row)
			}
		}
		payload["count"] = len(filtered)
		payload["data"] = filtered
		payload["driver_number"] = driverNumber
		return payload, nil
	}
	rows, err := u.repository.GetDriverDataset(ctx, sessionID, driverNumber, dataset)
	if err != nil {
		return nil, err
	}
	lookup, err := u.driverLookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		enrichDriverRow(row, lookup)
	}
	return map[string]any{
		"session_id":    sessionID,
		"driver_number": driverNumber,
		"dataset":       dataset,
		"count":         len(rows),
		"data":          rows,
		"metadata":      sessionMetadata(event, session),
	}, nil
}

// GetSessionFacts returns the requested session facts payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetSessionFacts(ctx context.Context, sessionID string) (map[string]any, error) {
	session, event, err := u.loadContext(ctx, sessionID)
	if err != nil || session == nil {
		return nil, err
	}
	raceControl, err := u.repository.GetSessionDataset(ctx, sessionID, "race_control")
	if err != nil {
		return nil, err
	}
	overtakes, err := u.repository.GetSessionDataset(ctx, sessionID, "overtakes")
	if err != nil {
		return nil, err
	}
	pitStops, err := u.repository.GetSessionDataset(ctx, sessionID, "pit")
	if err != nil {
		return nil, err
	}
	radio, err := u.repository.GetSessionDataset(ctx, sessionID, "team_radio")
	if err != nil {
		return nil, err
	}
	lookup, err := u.driverLookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	facts := make([]map[string]any, 0, len(raceControl)+len(overtakes)+len(pitStops)+len(radio))
	for _, row := range raceControl {
		enrichDriverRow(row, lookup)
		row["fact_type"] = "race_control"
		facts = append(facts, row)
	}
	for _, row := range overtakes {
		row["fact_type"] = "overtake"
		facts = append(facts, row)
	}
	for _, row := range pitStops {
		enrichDriverRow(row, lookup)
		row["fact_type"] = "pit_stop"
		facts = append(facts, row)
	}
	for _, row := range radio {
		enrichDriverRow(row, lookup)
		row["fact_type"] = "team_radio"
		facts = append(facts, row)
	}
	sort.Slice(facts, func(i, j int) bool {
		return stringifyFactDate(facts[i]) < stringifyFactDate(facts[j])
	})
	return map[string]any{
		"session_id": sessionID,
		"count":      len(facts),
		"data":       facts,
		"metadata":   sessionMetadata(event, session),
	}, nil
}

// GetDriverLapLocation returns the requested driver lap location payload for the supplied identifiers.
func (u *SessionQueryUseCase) GetDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (map[string]any, error) {
	session, event, err := u.loadContext(ctx, sessionID)
	if err != nil || session == nil {
		return nil, err
	}
	payload, err := u.repository.GetDriverLapLocation(ctx, sessionID, driverNumber, lapNumber)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{
			"session_id":    sessionID,
			"driver_number": driverNumber,
			"lap_number":    lapNumber,
			"count":         0,
			"data":          []map[string]any{},
			"metadata":      sessionMetadata(event, session),
		}, nil
	}
	lookup, err := u.driverLookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if rows, ok := payload["data"].([]map[string]any); ok {
		for _, row := range rows {
			enrichDriverRow(row, lookup)
		}
	}
	payload["metadata"] = sessionMetadata(event, session)
	return payload, nil
}

// loadContext implements the load context workflow for this package.
func (u *SessionQueryUseCase) loadContext(ctx context.Context, sessionID string) (*ports.ChampionshipSessionRef, *ports.ChampionshipEventRef, error) {
	session, err := u.client.GetSession(ctx, sessionID)
	if err != nil || session == nil {
		return session, nil, err
	}
	event, err := u.client.GetEvent(ctx, session.EventID)
	if err != nil {
		return nil, nil, err
	}
	return session, event, nil
}

// driverLookup implements the driver lookup workflow for this package.
func (u *SessionQueryUseCase) driverLookup(ctx context.Context, sessionID string) (map[int]ports.ChampionshipDriverRef, error) {
	drivers, err := u.client.ListSessionDrivers(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	lookup := make(map[int]ports.ChampionshipDriverRef, len(drivers))
	for _, driver := range drivers {
		lookup[driver.DriverNumber] = driver
	}
	return lookup, nil
}

// enrichDriverRow adds championship driver metadata to a race-data response row.
func enrichDriverRow(row map[string]any, lookup map[int]ports.ChampionshipDriverRef) {
	driverNumber := intFromAny(row["driver_number"])
	if driverNumber == 0 {
		return
	}
	driver, ok := lookup[driverNumber]
	if !ok {
		return
	}
	row["driver_number"] = driver.DriverNumber
	if _, exists := row["driver_name"]; !exists {
		row["driver_name"] = driver.DriverName
	}
	if _, exists := row["team_name"]; !exists {
		row["team_name"] = driver.TeamName
	}
	if driver.TeamColor != "" {
		row["team_color"] = driver.TeamColor
	}
}

// sessionMetadata combines event and session references into API response metadata.
func sessionMetadata(event *ports.ChampionshipEventRef, session *ports.ChampionshipSessionRef) map[string]any {
	if event == nil || session == nil {
		return map[string]any{"provider": "openf1"}
	}
	return map[string]any{
		"provider":          "openf1",
		"meeting_name":      event.Name,
		"country_name":      event.CountryName,
		"year":              event.SeasonYear,
		"meeting_key":       event.ExternalKey,
		"race_session_name": session.Name,
		"race_session_key":  session.ExternalKey,
	}
}

// isRemoteDataset reports whether a dataset should be proxied to championship-service.
func isRemoteDataset(dataset string) bool {
	switch dataset {
	case "session_result", "starting_grid", "championship_drivers", "championship_teams":
		return true
	default:
		return false
	}
}

// intFromAny converts common numeric payload values into an int.
func intFromAny(value any) int {
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
		if strings.TrimSpace(v) == "" {
			return 0
		}
		var parsed int
		fmt.Sscanf(v, "%d", &parsed)
		return parsed
	default:
		return 0
	}
}

// stringifyFactDate formats the best available fact timestamp for response ordering.
func stringifyFactDate(row map[string]any) string {
	if value, ok := row["date_utc"]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
	if value, ok := row["date"]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
	return ""
}
