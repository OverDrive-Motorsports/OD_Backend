/**
##
## OverDrive 2026
## All Technical rights reserved
##
## build_race.go - Core use case to build race archive data from OpenF1 resources.
##
*/

package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"overdrive/internal/domain"
	"overdrive/internal/providers/openf1"
)

// RaceBuilder aggregates provider data and builds the internal race archive payload.

type RaceBuildInput struct {
	Year         int
	CountryName  string
	MeetingName  string
	DriverNumber int
}

type RaceBuilder struct {
	client *openf1.Client
}

// NewRaceBuilder creates a use case instance that builds race archives from provider data.
func NewRaceBuilder(client *openf1.Client) *RaceBuilder {
	return &RaceBuilder{client: client}
}

// Build orchestrates OpenF1 data retrieval and assembles a domain race archive payload.
func (b *RaceBuilder) Build(ctx context.Context, in RaceBuildInput) (domain.RaceArchive, error) {
	meetings, err := b.client.Get(ctx, "meetings", map[string]string{
		"year":         strconv.Itoa(in.Year),
		"country_name": in.CountryName,
	})
	if err != nil {
		return domain.RaceArchive{}, fmt.Errorf("fetch meetings: %w", err)
	}
	if len(meetings) == 0 {
		return domain.RaceArchive{}, fmt.Errorf("no meetings found for %s %d", in.CountryName, in.Year)
	}

	meeting, err := pickMeeting(meetings, in.MeetingName)
	if err != nil {
		return domain.RaceArchive{}, err
	}

	meetingKey, err := extractInt(meeting, "meeting_key")
	if err != nil {
		return domain.RaceArchive{}, fmt.Errorf("meeting_key missing: %w", err)
	}

	sessions, err := b.client.Get(ctx, "sessions", map[string]string{
		"meeting_key": strconv.Itoa(meetingKey),
	})
	if err != nil {
		return domain.RaceArchive{}, fmt.Errorf("fetch sessions: %w", err)
	}
	if len(sessions) == 0 {
		return domain.RaceArchive{}, fmt.Errorf("no sessions found for meeting_key=%d", meetingKey)
	}

	raceSession, err := pickRaceSession(sessions)
	if err != nil {
		return domain.RaceArchive{}, err
	}

	sessionKey, err := extractInt(raceSession, "session_key")
	if err != nil {
		return domain.RaceArchive{}, fmt.Errorf("session_key missing for race session: %w", err)
	}

	endpoints := openf1.RaceSessionEndpoints
	if in.DriverNumber > 0 {
		endpoints = openf1.DriverFocusedEndpoints
	}

	datasets := make(map[string][]map[string]any, len(endpoints))
	counts := make(map[string]int, len(endpoints))
	fetchErrors := map[string]string{}

	// Fetch drivers first to power per-driver fallbacks for heavy datasets.
	driverRows, err := b.fetchDataset(ctx, "drivers", sessionKey, nil)
	if err != nil {
		return domain.RaceArchive{}, fmt.Errorf("fetch drivers: %w", err)
	}

	if in.DriverNumber > 0 {
		driverRows = filterDrivers(driverRows, in.DriverNumber)
		if len(driverRows) == 0 {
			return domain.RaceArchive{}, fmt.Errorf("driver_number=%d not found for session_key=%d", in.DriverNumber, sessionKey)
		}
	}

	datasets["drivers"] = driverRows
	counts["drivers"] = len(driverRows)
	driverNumbers := extractDriverNumbers(driverRows)

	for _, endpoint := range endpoints {
		if endpoint == "drivers" {
			continue
		}

		// Keep keys present even on failure to preserve a stable JSON shape.
		datasets[endpoint] = []map[string]any{}
		counts[endpoint] = 0

		rows, fetchErr := b.fetchDataset(ctx, endpoint, sessionKey, driverNumbers)
		if fetchErr != nil {
			fetchErrors[endpoint] = fetchErr.Error()
			continue
		}
		datasets[endpoint] = rows
		counts[endpoint] = len(rows)
	}

	archive := domain.RaceArchive{
		Metadata: domain.Metadata{
			GeneratedAt:  time.Now().UTC(),
			Provider:     "openf1",
			MeetingName:  extractStringOrDefault(meeting, "meeting_name", in.MeetingName),
			CountryName:  extractStringOrDefault(meeting, "country_name", in.CountryName),
			Year:         in.Year,
			MeetingKey:   meetingKey,
			RaceSession:  extractStringOrDefault(raceSession, "session_name", "Race"),
			RaceSessKey:  sessionKey,
			EndpointSize: len(endpoints),
			DriverNumber: in.DriverNumber,
		},
		Meeting:     meeting,
		AllSessions: sessions,
		RaceSession: raceSession,
		Datasets:    datasets,
		Counts:      counts,
		FetchErrors: fetchErrors,
	}

	if len(fetchErrors) == 0 {
		archive.FetchErrors = nil
	}

	return archive, nil
}

// fetchDataset retrieves one dataset, applying single-driver and fallback strategies when needed.
func (b *RaceBuilder) fetchDataset(ctx context.Context, endpoint string, sessionKey int, driverNumbers []int) ([]map[string]any, error) {
	if len(driverNumbers) == 1 && supportsDriverNumber(endpoint) {
		rows, err := b.client.Get(ctx, endpoint, map[string]string{
			"session_key":   strconv.Itoa(sessionKey),
			"driver_number": strconv.Itoa(driverNumbers[0]),
		})
		if err == nil {
			return rows, nil
		}

		if isNoResultsError(err) {
			return []map[string]any{}, nil
		}

		// If OpenF1 rejects driver filtering for this endpoint, retry session-wide.
		if !isBadRequestError(err) {
			if shouldFallbackByDriver(endpoint, err) {
				fallbackRows, fallbackErr := b.fetchDatasetByDriver(ctx, endpoint, sessionKey, driverNumbers)
				if fallbackErr != nil {
					return nil, fmt.Errorf("%s fallback by driver failed: %w", endpoint, fallbackErr)
				}
				return fallbackRows, nil
			}
			return nil, err
		}
	}

	rows, err := b.client.Get(ctx, endpoint, map[string]string{
		"session_key": strconv.Itoa(sessionKey),
	})
	if err == nil {
		return rows, nil
	}

	if isNoResultsError(err) {
		return []map[string]any{}, nil
	}

	if shouldFallbackByDriver(endpoint, err) {
		if len(driverNumbers) == 0 {
			return nil, fmt.Errorf("%s fallback impossible: no drivers found", endpoint)
		}

		fallbackRows, fallbackErr := b.fetchDatasetByDriver(ctx, endpoint, sessionKey, driverNumbers)
		if fallbackErr != nil {
			return nil, fmt.Errorf("%s fallback by driver failed: %w", endpoint, fallbackErr)
		}
		return fallbackRows, nil
	}

	return nil, err
}

// fetchDatasetByDriver aggregates an endpoint payload by querying each driver separately.
func (b *RaceBuilder) fetchDatasetByDriver(ctx context.Context, endpoint string, sessionKey int, driverNumbers []int) ([]map[string]any, error) {
	allRows := make([]map[string]any, 0, 4096)
	driverErrors := make([]string, 0)

	for _, driverNumber := range driverNumbers {
		rows, err := b.client.Get(ctx, endpoint, map[string]string{
			"session_key":   strconv.Itoa(sessionKey),
			"driver_number": strconv.Itoa(driverNumber),
		})
		if err != nil {
			if isNoResultsError(err) {
				continue
			}
			driverErrors = append(driverErrors, fmt.Sprintf("driver_number=%d: %v", driverNumber, err))
			continue
		}
		allRows = append(allRows, rows...)
	}

	if len(driverErrors) > 0 {
		if len(allRows) == 0 {
			return nil, errors.New(strings.Join(driverErrors, " | "))
		}
		// Fail in strict mode to flag partially recovered payloads.
		return nil, fmt.Errorf("partial data recovered (%d rows), but some drivers failed: %s", len(allRows), strings.Join(driverErrors, " | "))
	}

	return allRows, nil
}

// shouldFallbackByDriver determines whether a failed endpoint should be retried per driver.
func shouldFallbackByDriver(endpoint string, err error) bool {
	if endpoint != "car_data" && endpoint != "location" {
		return false
	}
	return isTooMuchDataError(err)
}

// isTooMuchDataError detects OpenF1 "too much data" errors from textual error details.
func isTooMuchDataError(err error) bool {
	raw := strings.ToLower(err.Error())
	return strings.Contains(raw, "status=422") && strings.Contains(raw, "too much data")
}

// isNoResultsError detects OpenF1 "no results found" responses.
func isNoResultsError(err error) bool {
	raw := strings.ToLower(err.Error())
	return strings.Contains(raw, "status=404") && strings.Contains(raw, "no results found")
}

// isBadRequestError detects HTTP 400 provider responses.
func isBadRequestError(err error) bool {
	raw := strings.ToLower(err.Error())
	return strings.Contains(raw, "status=400")
}

// supportsDriverNumber reports whether an endpoint accepts the driver_number filter.
func supportsDriverNumber(endpoint string) bool {
	switch endpoint {
	case "drivers",
		"laps",
		"car_data",
		"location",
		"position",
		"intervals",
		"stints",
		"pit",
		"team_radio",
		"session_result",
		"starting_grid",
		"overtakes",
		"championship_drivers":
		return true
	default:
		return false
	}
}

// extractDriverNumbers builds a sorted unique list of driver numbers from driver rows.
func extractDriverNumbers(drivers []map[string]any) []int {
	out := make([]int, 0, len(drivers))
	seen := make(map[int]struct{}, len(drivers))

	for _, d := range drivers {
		num, err := extractInt(d, "driver_number")
		if err != nil || num <= 0 {
			continue
		}
		if _, exists := seen[num]; exists {
			continue
		}
		seen[num] = struct{}{}
		out = append(out, num)
	}

	slices.Sort(out)
	return out
}

// filterDrivers keeps only rows matching a specific driver number.
func filterDrivers(drivers []map[string]any, driverNumber int) []map[string]any {
	filtered := make([]map[string]any, 0, 1)
	for _, d := range drivers {
		num, err := extractInt(d, "driver_number")
		if err != nil {
			continue
		}
		if num == driverNumber {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// pickMeeting selects the target meeting using exact and fuzzy name matching.
func pickMeeting(meetings []map[string]any, meetingName string) (map[string]any, error) {
	if len(meetings) == 0 {
		return nil, errors.New("empty meetings list")
	}
	if meetingName == "" {
		return meetings[0], nil
	}

	needle := strings.ToLower(strings.TrimSpace(meetingName))

	for _, m := range meetings {
		name := strings.ToLower(extractStringOrDefault(m, "meeting_name", ""))
		if name == needle {
			return m, nil
		}
	}

	for _, m := range meetings {
		official := strings.ToLower(extractStringOrDefault(m, "meeting_official_name", ""))
		if strings.Contains(official, needle) {
			return m, nil
		}
	}

	for _, m := range meetings {
		name := strings.ToLower(extractStringOrDefault(m, "meeting_name", ""))
		if strings.Contains(name, needle) {
			return m, nil
		}
	}

	if len(meetings) == 1 {
		return meetings[0], nil
	}

	return nil, fmt.Errorf("meeting %q not found", meetingName)
}

// pickRaceSession selects the race session entry from a meeting's sessions list.
func pickRaceSession(sessions []map[string]any) (map[string]any, error) {
	if len(sessions) == 0 {
		return nil, errors.New("empty sessions list")
	}

	sorted := slices.Clone(sessions)
	slices.SortFunc(sorted, func(a, b map[string]any) int {
		as := extractStringOrDefault(a, "date_start", "")
		bs := extractStringOrDefault(b, "date_start", "")
		return strings.Compare(as, bs)
	})

	for _, s := range sorted {
		if strings.EqualFold(extractStringOrDefault(s, "session_name", ""), "Race") {
			return s, nil
		}
	}

	for _, s := range sorted {
		name := strings.ToLower(extractStringOrDefault(s, "session_name", ""))
		if strings.Contains(name, "race") {
			return s, nil
		}
	}

	return nil, errors.New("race session not found")
}

// extractStringOrDefault returns a non-empty string field or a fallback value.
func extractStringOrDefault(m map[string]any, key, fallback string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return fallback
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// extractInt converts a loosely typed map value to an integer.
func extractInt(m map[string]any, key string) (int, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("%s not found", key)
	}

	switch val := v.(type) {
	case float64:
		return int(val), nil
	case float32:
		return int(val), nil
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return 0, fmt.Errorf("invalid %s: %w", key, err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("unsupported type for %s (%T)", key, v)
	}
}
