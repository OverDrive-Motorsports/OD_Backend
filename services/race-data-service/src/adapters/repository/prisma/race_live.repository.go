/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_live.repository.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
##
*/

package prismaadapter

import (
	"context"
	"sort"
	"time"

	db "overdrive/services/race-data-service/resources/db"
	"overdrive/services/race-data-service/src/core/domain"
)

type RaceLiveRepository struct {
	client *db.PrismaClient
}

// NewRaceLiveRepository builds and returns a race live repository with its required dependencies.
func NewRaceLiveRepository(client *db.PrismaClient) *RaceLiveRepository {
	return &RaceLiveRepository{client: client}
}

// ListPositions returns live race position samples, optionally filtered by driver
// and/or lap number. Only the latest sample per driver (within scope) is returned.
func (r *RaceLiveRepository) ListPositions(ctx context.Context, sessionID string, driverNumber *int, lapNumber *int) ([]domain.RacePosition, error) {
	params := []db.RacePositionSampleWhereParam{db.RacePositionSample.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.RacePositionSample.DriverNumber.Equals(*driverNumber))
	}
	if lapNumber != nil {
		params = append(params, db.RacePositionSample.LapNumber.Equals(*lapNumber))
	}
	rows, err := r.client.RacePositionSample.FindMany(params...).OrderBy(db.RacePositionSample.DateUtc.Order(db.SortOrderDesc)).Exec(ctx)
	if err != nil {
		return nil, err
	}

	intervals, err := r.intervalLookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	lapsCompleted, err := r.lapsCompletedLookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	latestByDriver := make(map[int]db.RacePositionSampleModel)
	for _, row := range rows {
		if _, seen := latestByDriver[row.DriverNumber]; !seen {
			latestByDriver[row.DriverNumber] = row
		}
	}

	items := make([]domain.RacePosition, 0, len(latestByDriver))
	for driver, row := range latestByDriver {
		position := 0
		if p, ok := row.Position(); ok {
			position = p
		}
		item := domain.RacePosition{
			DriverNumber:  driver,
			Position:      position,
			LapsCompleted: lapsCompleted[driver],
			Timestamp:     time.Time(row.DateUtc),
		}
		if interval, ok := intervals[driver]; ok {
			item.GapToLeader = interval.gapToLeader
			if interval.gapAhead != "" {
				gapAhead := interval.gapAhead
				item.GapAhead = &gapAhead
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	return items, nil
}

// ListDriverNumbers returns the distinct driver numbers with lap data for a session.
func (r *RaceLiveRepository) ListDriverNumbers(ctx context.Context, sessionID string) ([]int, error) {
	rows, err := r.client.RaceLap.FindMany(db.RaceLap.SessionID.Equals(sessionID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[int]struct{})
	drivers := make([]int, 0)
	for _, row := range rows {
		if _, ok := seen[row.DriverNumber]; ok {
			continue
		}
		seen[row.DriverNumber] = struct{}{}
		drivers = append(drivers, row.DriverNumber)
	}
	sort.Ints(drivers)
	return drivers, nil
}

// ListLapsForDriver returns a driver's laps, optionally restricted to a single lap
// number, along with the best/average lap computed from ALL of that driver's laps.
func (r *RaceLiveRepository) ListLapsForDriver(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) (domain.RaceLapsResponse, error) {
	rows, err := r.client.RaceLap.FindMany(
		db.RaceLap.SessionID.Equals(sessionID),
		db.RaceLap.DriverNumber.Equals(driverNumber),
	).Exec(ctx)
	if err != nil {
		return domain.RaceLapsResponse{}, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].LapNumber < rows[j].LapNumber })

	var best, sum float64
	var completedLaps int
	entries := make([]domain.RaceLapEntry, 0, len(rows))
	for _, row := range rows {
		duration, hasDuration := row.LapDurationSec()
		if hasDuration {
			sum += duration
			completedLaps++
			if best == 0 || duration < best {
				best = duration
			}
		}
		if lapNumber != nil && row.LapNumber != *lapNumber {
			continue
		}
		entry := domain.RaceLapEntry{LapNumber: row.LapNumber}
		if hasDuration {
			d := duration
			entry.LapDuration = &d
		}
		if s1, ok := row.Sector1Sec(); ok {
			entry.Sector1 = &s1
		}
		if s2, ok := row.Sector2Sec(); ok {
			entry.Sector2 = &s2
		}
		if s3, ok := row.Sector3Sec(); ok {
			entry.Sector3 = &s3
		}
		if pitOut, ok := row.IsPitOutLap(); ok {
			entry.IsPitOutLap = pitOut
		}
		entries = append(entries, entry)
	}

	response := domain.RaceLapsResponse{DriverNumber: driverNumber, Laps: entries}
	if completedLaps > 0 {
		bestLap := best
		averageLap := sum / float64(completedLaps)
		response.BestLap = &bestLap
		response.AverageLap = &averageLap
	}
	return response, nil
}

// ListStints returns tyre stints, optionally filtered by driver.
func (r *RaceLiveRepository) ListStints(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceStint, error) {
	params := []db.RaceStintWhereParam{db.RaceStint.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.RaceStint.DriverNumber.Equals(*driverNumber))
	}
	rows, err := r.client.RaceStint.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RaceStint, 0, len(rows))
	for _, row := range rows {
		stintNumber, _ := row.StintNumber()
		lapStart, _ := row.LapStart()
		lapEnd, _ := row.LapEnd()
		compound, _ := row.Compound()
		tyreAge, _ := row.TyreAgeLapsStart()
		items = append(items, domain.RaceStint{
			DriverNumber:   row.DriverNumber,
			StintNumber:    stintNumber,
			Compound:       compound,
			LapStart:       lapStart,
			LapEnd:         lapEnd,
			TyreAgeAtStart: tyreAge,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DriverNumber != items[j].DriverNumber {
			return items[i].DriverNumber < items[j].DriverNumber
		}
		return items[i].StintNumber < items[j].StintNumber
	})
	return items, nil
}

// ListPitStops returns pit stops, optionally filtered by driver.
func (r *RaceLiveRepository) ListPitStops(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RacePitStop, error) {
	params := []db.RacePitStopWhereParam{db.RacePitStop.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.RacePitStop.DriverNumber.Equals(*driverNumber))
	}
	rows, err := r.client.RacePitStop.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RacePitStop, 0, len(rows))
	for _, row := range rows {
		lapNumber, _ := row.LapNumber()
		duration, _ := row.PitDurationSec()
		items = append(items, domain.RacePitStop{
			DriverNumber: row.DriverNumber,
			LapNumber:    lapNumber,
			PitDuration:  duration,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DriverNumber != items[j].DriverNumber {
			return items[i].DriverNumber < items[j].DriverNumber
		}
		return items[i].LapNumber < items[j].LapNumber
	})
	return items, nil
}

// ListWeather returns all weather samples for a session, oldest first.
func (r *RaceLiveRepository) ListWeather(ctx context.Context, sessionID string) ([]domain.RaceWeatherSample, error) {
	rows, err := r.client.RaceWeatherSample.FindMany(db.RaceWeatherSample.SessionID.Equals(sessionID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RaceWeatherSample, 0, len(rows))
	for _, row := range rows {
		item := domain.RaceWeatherSample{Timestamp: time.Time(row.DateUtc)}
		if v, ok := row.AirTempC(); ok {
			item.AirTemperature = &v
		}
		if v, ok := row.TrackTempC(); ok {
			item.TrackTemperature = &v
		}
		if v, ok := row.HumidityPct(); ok {
			item.Humidity = &v
		}
		if v, ok := row.WindSpeedKph(); ok {
			item.WindSpeed = &v
		}
		if v, ok := row.RainfallMm(); ok {
			item.Rainfall = v > 0
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp.Before(items[j].Timestamp) })
	return items, nil
}

// ListRadio returns team radio messages, optionally filtered by driver.
func (r *RaceLiveRepository) ListRadio(ctx context.Context, sessionID string, driverNumber *int) ([]domain.RaceRadioMessage, error) {
	params := []db.TeamRadioMessageWhereParam{db.TeamRadioMessage.SessionID.Equals(sessionID)}
	if driverNumber != nil {
		params = append(params, db.TeamRadioMessage.DriverNumber.Equals(*driverNumber))
	}
	rows, err := r.client.TeamRadioMessage.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.RaceRadioMessage, 0, len(rows))
	for _, row := range rows {
		item := domain.RaceRadioMessage{DriverNumber: row.DriverNumber}
		if dateUtc, ok := row.DateUtc(); ok {
			item.Timestamp = time.Time(dateUtc)
		}
		if url, ok := row.RecordingURL(); ok {
			item.AudioURL = url
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp.Before(items[j].Timestamp) })
	return items, nil
}

type intervalRow struct {
	gapToLeader string
	gapAhead    string
}

// intervalLookup builds a driverNumber -> latest interval sample lookup for a session.
func (r *RaceLiveRepository) intervalLookup(ctx context.Context, sessionID string) (map[int]intervalRow, error) {
	rows, err := r.client.RaceIntervalSample.FindMany(
		db.RaceIntervalSample.SessionID.Equals(sessionID),
	).OrderBy(db.RaceIntervalSample.DateUtc.Order(db.SortOrderDesc)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	lookup := make(map[int]intervalRow)
	for _, row := range rows {
		if _, seen := lookup[row.DriverNumber]; seen {
			continue
		}
		gapToLeader, _ := row.GapToLeader()
		gapAhead, _ := row.IntervalToFrontDriver()
		lookup[row.DriverNumber] = intervalRow{gapToLeader: gapToLeader, gapAhead: gapAhead}
	}
	return lookup, nil
}

// lapsCompletedLookup builds a driverNumber -> completed lap count lookup for a session.
func (r *RaceLiveRepository) lapsCompletedLookup(ctx context.Context, sessionID string) (map[int]int, error) {
	rows, err := r.client.RaceLap.FindMany(db.RaceLap.SessionID.Equals(sessionID)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[int]int)
	for _, row := range rows {
		if _, hasDuration := row.LapDurationSec(); hasDuration {
			counts[row.DriverNumber]++
		}
	}
	return counts, nil
}

// lapTimeWindow resolves the [start, end) UTC time window covered by a given lap,
// mirroring the logic used by GetDriverLapLocation in query.repository.go.
func lapTimeWindow(ctx context.Context, client *db.PrismaClient, sessionID string, driverNumber int, lapNumber int) (time.Time, time.Time, bool, error) {
	lap, err := client.RaceLap.FindUnique(
		db.RaceLap.SessionIDDriverNumberLapNumber(
			db.RaceLap.SessionID.Equals(sessionID),
			db.RaceLap.DriverNumber.Equals(driverNumber),
			db.RaceLap.LapNumber.Equals(lapNumber),
		),
	).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return time.Time{}, time.Time{}, false, nil
		}
		return time.Time{}, time.Time{}, false, err
	}
	if lap == nil {
		return time.Time{}, time.Time{}, false, nil
	}
	startedAt, ok := lap.DateStartUtc()
	if !ok {
		return time.Time{}, time.Time{}, false, nil
	}
	startTime := time.Time(startedAt)
	endTime := startTime
	if lapDuration, ok := lap.LapDurationSec(); ok {
		endTime = startTime.Add(time.Duration(lapDuration * float64(time.Second)))
	} else {
		nextLap, nextErr := client.RaceLap.FindUnique(
			db.RaceLap.SessionIDDriverNumberLapNumber(
				db.RaceLap.SessionID.Equals(sessionID),
				db.RaceLap.DriverNumber.Equals(driverNumber),
				db.RaceLap.LapNumber.Equals(lapNumber+1),
			),
		).Exec(ctx)
		if nextErr == nil && nextLap != nil {
			if nextStart, nextOK := nextLap.DateStartUtc(); nextOK {
				endTime = time.Time(nextStart)
			}
		}
	}
	return startTime, endTime, true, nil
}
