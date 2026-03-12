/**
##
## OverDrive 2026
## All Technical rights reserved
##
## fixtures.go - Shared deterministic fixtures used across unit test packages.
##
*/

package mocks

import (
	"time"

	"overdrive/internal/domain"
)

// SampleArchive returns a stable merged race archive fixture for HTTP and service tests.
func SampleArchive() domain.RaceArchive {
	generatedAt := time.Date(2025, 3, 16, 6, 5, 0, 0, time.UTC)

	return domain.RaceArchive{
		Metadata: domain.Metadata{
			GeneratedAt:  generatedAt,
			Provider:     "openf1",
			MeetingName:  "Australian Grand Prix",
			CountryName:  "Australia",
			Year:         2025,
			MeetingKey:   1254,
			RaceSession:  "Race",
			RaceSessKey:  9693,
			EndpointSize: 16,
		},
		Meeting: map[string]any{
			"meeting_key":  1254,
			"meeting_name": "Australian Grand Prix",
		},
		RaceSession: map[string]any{
			"session_key":  9693,
			"session_name": "Race",
		},
		AllSessions: []map[string]any{
			{
				"session_key":  9691,
				"session_name": "Practice 1",
			},
			{
				"session_key":  9693,
				"session_name": "Race",
			},
		},
		Datasets: map[string][]map[string]any{
			"drivers": {
				{
					"driver_number":  63,
					"full_name":      "George Russell",
					"broadcast_name": "G RUSSELL",
					"team_name":      "Mercedes",
					"team_colour":    "00D2BE",
				},
				{
					"driver_number":  1,
					"full_name":      "Max Verstappen",
					"broadcast_name": "M VERSTAPPEN",
					"team_name":      "Red Bull Racing",
					"team_colour":    "3671C6",
				},
			},
			"laps": {
				{"driver_number": 63, "lap_number": 1, "date_start": "2025-03-16T04:00:00Z", "lap_duration": 90.0},
				{"driver_number": 63, "lap_number": 2, "date_start": "2025-03-16T04:01:30Z", "lap_duration": 91.0},
				{"driver_number": 1, "lap_number": 1, "date_start": "2025-03-16T04:00:00Z", "lap_duration": 89.5},
			},
			"car_data": {
				{"driver_number": 63, "date": "2025-03-16T04:00:00Z", "speed": 298},
				{"driver_number": 1, "date": "2025-03-16T04:00:00Z", "speed": 301},
			},
			"location": {
				{"driver_number": 63, "date": "2025-03-16T04:00:10Z", "x": 1, "y": 2, "z": 3},
				{"driver_number": 63, "date": "2025-03-16T04:01:00Z", "x": 7, "y": 8, "z": 9},
				{"driver_number": 63, "date": "2025-03-16T04:01:40Z", "x": 10, "y": 11, "z": 12},
				{"driver_number": 1, "date": "2025-03-16T04:00:00Z", "x": 4, "y": 5, "z": 6},
			},
			"position": {
				{"driver_number": 63, "position": 2, "date": "2025-03-16T04:10:00Z"},
				{"driver_number": 1, "position": 1, "date": "2025-03-16T04:10:00Z"},
				{"driver_number": 63, "position": 1, "date": "2025-03-16T04:20:00Z"},
				{"driver_number": 1, "position": 2, "date": "2025-03-16T04:20:00Z"},
			},
			"intervals": {
				{"driver_number": 63, "date": "2025-03-16T04:20:00Z", "gap_to_leader": "LEADER"},
				{"driver_number": 1, "date": "2025-03-16T04:20:00Z", "gap_to_leader": "+1.200"},
			},
			"stints": {
				{"driver_number": 63, "stint_number": 1, "compound": "MEDIUM"},
				{"driver_number": 1, "stint_number": 1, "compound": "HARD"},
			},
			"pit": {
				{"driver_number": 63, "lap_number": 20},
			},
			"team_radio": {
				{"driver_number": 1, "date": "2025-03-16T04:05:00Z", "recording_url": "https://radio.local/max.mp3"},
			},
			"race_control": {
				{"date": "2025-03-16T04:06:00Z", "category": "Flag", "flag": "YELLOW"},
			},
			"weather": {
				{"date": "2025-03-16T04:00:00Z", "air_temperature": 22.5},
			},
			"session_result": {
				{"driver_number": 63, "position": 1, "points": 25},
				{"driver_number": 1, "position": 2, "points": 18},
			},
			"starting_grid": {
				{"driver_number": 63, "grid_position": 2},
				{"driver_number": 1, "grid_position": 1},
			},
			"overtakes": {
				{"date": "2025-03-16T04:15:00Z", "lap_number": 10, "overtaking_driver_number": 63, "overtaken_driver_number": 1},
			},
			"championship_drivers": {
				{"driver_number": 63, "position_current": 1, "points_current": 25},
				{"driver_number": 1, "position_current": 2, "points_current": 18},
			},
			"championship_teams": {
				{"team_name": "Mercedes", "position_current": 1, "points_current": 25},
				{"team_name": "Red Bull Racing", "position_current": 2, "points_current": 18},
			},
		},
		Counts: map[string]int{
			"drivers":              2,
			"laps":                 3,
			"car_data":             2,
			"location":             4,
			"position":             4,
			"intervals":            2,
			"stints":               2,
			"pit":                  1,
			"team_radio":           1,
			"race_control":         1,
			"weather":              1,
			"session_result":       2,
			"starting_grid":        2,
			"overtakes":            1,
			"championship_drivers": 2,
			"championship_teams":   2,
		},
	}
}

// SampleChampionship returns a stable championship summary fixture.
func SampleChampionship() domain.ChampionshipSummary {
	return domain.ChampionshipSummary{
		ID:        "champ-f1",
		Code:      "f1",
		Name:      "Formula 1",
		Category:  "single-seater",
		IsActive:  true,
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// SampleEvent returns a stable event summary fixture.
func SampleEvent() domain.EventSummary {
	endsAt := time.Date(2025, 3, 16, 6, 30, 0, 0, time.UTC)
	round := 1

	return domain.EventSummary{
		ID:             "event-aus-2025",
		ChampionshipID: "champ-f1",
		SeasonYear:     2025,
		RoundNumber:    &round,
		Name:           "Australian Grand Prix",
		OfficialName:   "Formula 1 Louis Vuitton Australian Grand Prix 2025",
		CountryName:    "Australia",
		CountryCode:    "AUS",
		CircuitName:    "Albert Park",
		ExternalKey:    "1254",
		Status:         "finished",
		StartsAtUTC:    time.Date(2025, 3, 16, 4, 0, 0, 0, time.UTC),
		EndsAtUTC:      &endsAt,
		CreatedAt:      time.Date(2025, 3, 16, 0, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2025, 3, 16, 6, 31, 0, 0, time.UTC),
	}
}

// SampleSession returns a stable session summary fixture.
func SampleSession() domain.SessionSummary {
	storedAt := time.Date(2025, 3, 16, 6, 5, 0, 0, time.UTC)
	endedAt := time.Date(2025, 3, 16, 6, 30, 0, 0, time.UTC)

	return domain.SessionSummary{
		ID:           "session-race-9693",
		EventID:      "event-aus-2025",
		Type:         "race",
		Status:       "finished",
		Name:         "Race",
		ExternalKey:  "9693",
		BroadcastURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ&session=9693",
		StartedAtUTC: time.Date(2025, 3, 16, 4, 0, 0, 0, time.UTC),
		EndedAtUTC:   &endedAt,
		LatestCounts: SampleArchive().Counts,
		LatestStored: &storedAt,
		CreatedAt:    time.Date(2025, 3, 16, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2025, 3, 16, 6, 31, 0, 0, time.UTC),
	}
}
