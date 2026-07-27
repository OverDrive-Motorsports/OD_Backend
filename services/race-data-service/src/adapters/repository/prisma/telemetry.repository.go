/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## telemetry.repository.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
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

type TelemetryRepository struct {
	client *db.PrismaClient
}

// NewTelemetryRepository builds and returns a telemetry repository with its required dependencies.
func NewTelemetryRepository(client *db.PrismaClient) *TelemetryRepository {
	return &TelemetryRepository{client: client}
}

// GetSpeed returns every speed sample for a driver, oldest first, optionally
// scoped to a lap. Changed 2026-07-08 from an aggregated current/top/average
// snapshot to the full raw sample list — see domain.TelemetrySpeed.
func (r *TelemetryRepository) GetSpeed(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetrySpeed, error) {
	rows, err := r.telemetrySamples(ctx, sessionID, driverNumber, lapNumber)
	if err != nil {
		return nil, err
	}
	items := make([]domain.TelemetrySpeed, 0, len(rows))
	for _, row := range rows {
		speed, _ := row.SpeedKph()
		gear, _ := row.Gear()
		items = append(items, domain.TelemetrySpeed{
			Speed:     speed,
			Gear:      gear,
			Timestamp: time.Time(row.DateUtc),
		})
	}
	return items, nil
}

// GetEngine returns every engine sample for a driver, oldest first, optionally
// scoped to a lap. Changed 2026-07-08 from a "latest sample only" snapshot to
// the full raw sample list — see domain.TelemetryEngine.
func (r *TelemetryRepository) GetEngine(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryEngine, error) {
	rows, err := r.telemetrySamples(ctx, sessionID, driverNumber, lapNumber)
	if err != nil {
		return nil, err
	}
	items := make([]domain.TelemetryEngine, 0, len(rows))
	for _, row := range rows {
		rpm, _ := row.Rpm()
		gear, _ := row.Gear()
		throttle, _ := row.ThrottlePct()
		brake, _ := row.BrakePct()
		drsState, _ := row.DrsState()
		items = append(items, domain.TelemetryEngine{
			Rpm:             rpm,
			Gear:            gear,
			ThrottlePercent: throttle,
			BrakePercent:    brake,
			DrsActive:       drsState >= 10,
			Timestamp:       time.Time(row.DateUtc),
		})
	}
	return items, nil
}

// ListLocation returns spatial samples for a driver, optionally scoped to a lap.
func (r *TelemetryRepository) ListLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]domain.TelemetryLocation, error) {
	params := []db.RaceLocationSampleWhereParam{
		db.RaceLocationSample.SessionID.Equals(sessionID),
		db.RaceLocationSample.DriverNumber.Equals(driverNumber),
	}
	if lapNumber != nil {
		start, end, ok, err := lapTimeWindow(ctx, r.client, sessionID, driverNumber, *lapNumber)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []domain.TelemetryLocation{}, nil
		}
		params = append(params,
			db.RaceLocationSample.DateUtc.Gte(db.DateTime(start)),
			db.RaceLocationSample.DateUtc.Lt(db.DateTime(end)),
		)
	}
	rows, err := r.client.RaceLocationSample.FindMany(params...).Exec(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.TelemetryLocation, 0, len(rows))
	for _, row := range rows {
		x, _ := row.X()
		y, _ := row.Y()
		z, _ := row.Z()
		items = append(items, domain.TelemetryLocation{
			X:         x,
			Y:         y,
			Z:         z,
			Timestamp: time.Time(row.DateUtc),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp.Before(items[j].Timestamp) })
	return items, nil
}

// GetIntervals returns the latest interval sample for a driver.
func (r *TelemetryRepository) GetIntervals(ctx context.Context, sessionID string, driverNumber int) (*domain.TelemetryIntervals, error) {
	rows, err := r.client.RaceIntervalSample.FindMany(
		db.RaceIntervalSample.SessionID.Equals(sessionID),
		db.RaceIntervalSample.DriverNumber.Equals(driverNumber),
	).OrderBy(db.RaceIntervalSample.DateUtc.Order(db.SortOrderDesc)).Take(1).Exec(ctx)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	latest := rows[0]
	gapToLeader, _ := latest.GapToLeader()
	gapAheadValue, hasGapAhead := latest.IntervalToFrontDriver()
	result := &domain.TelemetryIntervals{
		DriverNumber: driverNumber,
		GapToLeader:  gapToLeader,
		Timestamp:    time.Time(latest.DateUtc),
	}
	if hasGapAhead {
		result.GapAhead = &gapAheadValue
	}
	return result, nil
}

// telemetrySamples loads a driver's telemetry samples, oldest first, optionally
// scoped to a single lap's time window.
func (r *TelemetryRepository) telemetrySamples(ctx context.Context, sessionID string, driverNumber int, lapNumber *int) ([]db.RaceTelemetrySampleModel, error) {
	params := []db.RaceTelemetrySampleWhereParam{
		db.RaceTelemetrySample.SessionID.Equals(sessionID),
		db.RaceTelemetrySample.DriverNumber.Equals(driverNumber),
	}
	if lapNumber != nil {
		start, end, ok, err := lapTimeWindow(ctx, r.client, sessionID, driverNumber, *lapNumber)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		params = append(params,
			db.RaceTelemetrySample.DateUtc.Gte(db.DateTime(start)),
			db.RaceTelemetrySample.DateUtc.Lt(db.DateTime(end)),
		)
	}
	rows, err := r.client.RaceTelemetrySample.FindMany(params...).OrderBy(db.RaceTelemetrySample.DateUtc.Order(db.SortOrderAsc)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
