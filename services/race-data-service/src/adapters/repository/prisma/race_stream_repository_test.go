/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_stream_repository_test.go - Package prismaadapter source file for services/race-data-service/src/adapters/repository/prisma.
##
*/

package prismaadapter

import (
	"testing"
	"time"

	db "overdrive/services/race-data-service/resources/db"
)

// These tests exercise ListTelemetryFrames/ListStintFrames/ListLapFrames's pure helper functions
// directly, without a database, by hand-constructing db.*Model structs. This works because
// db.DateTime is a type alias for time.Time (see runtime/types/types.go in
// steebchen/prisma-client-go) and every field these helpers read is exported on the model's
// embedded Inner* struct - no generated accessor requires an actual Prisma client/connection.

func t0(offsetSeconds int) time.Time {
	base := time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetSeconds) * time.Second)
}

func locationRow(driverNumber int, offsetSeconds int, x, y, z float64) db.RaceLocationSampleModel {
	return db.RaceLocationSampleModel{InnerRaceLocationSample: db.InnerRaceLocationSample{
		DriverNumber: driverNumber,
		DateUtc:      db.DateTime(t0(offsetSeconds)),
		X:            &x,
		Y:            &y,
		Z:            &z,
	}}
}

func lapRow(driverNumber, lapNumber int, startOffsetSeconds int, hasStart bool) db.RaceLapModel {
	inner := db.InnerRaceLap{DriverNumber: driverNumber, LapNumber: lapNumber}
	if hasStart {
		start := db.DateTime(t0(startOffsetSeconds))
		inner.DateStartUtc = &start
	}
	return db.RaceLapModel{InnerRaceLap: inner}
}

// TestNearestLocation_AdvancesToClosestSample proves nearestLocation's forward-only pointer walk
// finds the closest-by-absolute-distance sample as t increases, and never reconsiders samples it
// has already passed.
func TestNearestLocation_AdvancesToClosestSample(t *testing.T) {
	rows := []db.RaceLocationSampleModel{
		locationRow(63, 0, 1, 1, 1),
		locationRow(63, 10, 2, 2, 2),
		locationRow(63, 20, 3, 3, 3),
	}
	idx := 0

	// t=1 is closest to sample 0 (offset 0).
	x, y, z := nearestLocation(rows, &idx, t0(1))
	if x != 1 || y != 1 || z != 1 || idx != 0 {
		t.Fatalf("expected sample 0 (1,1,1) idx=0, got (%v,%v,%v) idx=%d", x, y, z, idx)
	}

	// t=9 is closer to sample 1 (offset 10) than sample 0 (offset 0): distance 1 vs 9.
	x, y, z = nearestLocation(rows, &idx, t0(9))
	if x != 2 || y != 2 || z != 2 || idx != 1 {
		t.Fatalf("expected sample 1 (2,2,2) idx=1, got (%v,%v,%v) idx=%d", x, y, z, idx)
	}

	// t=25 (past the last sample) sticks on the last sample (idx never exceeds len-1).
	x, y, z = nearestLocation(rows, &idx, t0(25))
	if x != 3 || y != 3 || z != 3 || idx != 2 {
		t.Fatalf("expected sample 2 (3,3,3) idx=2, got (%v,%v,%v) idx=%d", x, y, z, idx)
	}
}

// TestNearestLocation_EmptyRows proves an empty location table returns zero values instead of
// panicking on an index into an empty slice.
func TestNearestLocation_EmptyRows(t *testing.T) {
	idx := 0
	x, y, z := nearestLocation(nil, &idx, t0(0))
	if x != 0 || y != 0 || z != 0 {
		t.Fatalf("expected zero values for empty rows, got (%v,%v,%v)", x, y, z)
	}
}

// TestCurrentLapNumber_FloorSemantics proves currentLapNumber uses "last lap whose start is <= t"
// (a step function), not nearest-by-distance - a lap number must never be reported before that
// lap has actually started, even if the next lap's start is closer in absolute time.
func TestCurrentLapNumber_FloorSemantics(t *testing.T) {
	rows := []db.RaceLapModel{
		lapRow(63, 1, 0, true),
		lapRow(63, 2, 100, true),
		lapRow(63, 3, 200, true),
	}
	idx := 0

	// t=50 is closer to lap 2's start (100) than lap 1's start (0) by distance, but lap 2 hasn't
	// started yet at t=50 - floor semantics must keep reporting lap 1.
	if got := currentLapNumber(rows, &idx, t0(50)); got != 1 {
		t.Fatalf("expected lap 1 (floor) at t=50, got %d", got)
	}

	// t=150: lap 2 has started (100 <= 150 < 200), so it becomes current.
	if got := currentLapNumber(rows, &idx, t0(150)); got != 2 {
		t.Fatalf("expected lap 2 at t=150, got %d", got)
	}

	// t=500 (past the last lap's start): sticks on the last known lap.
	if got := currentLapNumber(rows, &idx, t0(500)); got != 3 {
		t.Fatalf("expected lap 3 (last known) at t=500, got %d", got)
	}
}

// TestCurrentLapNumber_SkipsLapsWithNoRecordedStart proves a lap with no dateStartUtc is never
// used as the next boundary. Since the walk only ever looks one row ahead (idx+1), a gap
// permanently blocks advancing past it in a single call chain - the previous known lap number
// (here, lap 1) sticks even once a later lap's own start time has passed, exactly as documented
// on currentLapNumber ("a gap in that data just keeps the previous known lap number instead of
// guessing").
func TestCurrentLapNumber_SkipsLapsWithNoRecordedStart(t *testing.T) {
	rows := []db.RaceLapModel{
		lapRow(63, 1, 0, true),
		lapRow(63, 2, 100, false), // no recorded start - must never become "current"
		lapRow(63, 3, 200, true),
	}
	idx := 0

	if got := currentLapNumber(rows, &idx, t0(150)); got != 1 {
		t.Fatalf("expected lap 1 to stick (lap 2 has no recorded start), got %d", got)
	}
	if got := currentLapNumber(rows, &idx, t0(250)); got != 1 {
		t.Fatalf("expected lap 1 to keep sticking even past lap 3's start, since the gap at lap 2 blocks the one-ahead walk, got %d", got)
	}
}

// TestCurrentLapNumber_EmptyRows proves an empty lap table returns 0 instead of panicking.
func TestCurrentLapNumber_EmptyRows(t *testing.T) {
	idx := 0
	if got := currentLapNumber(nil, &idx, t0(0)); got != 0 {
		t.Fatalf("expected 0 for empty rows, got %d", got)
	}
}

// TestCloserTo covers both branches of the <= comparison, including the tie case (candidate wins
// ties, matching nearestLocation's "advance while closerTo" loop condition).
func TestCloserTo(t *testing.T) {
	target := t0(10)
	candidateCloser := db.DateTime(t0(11))
	current := db.DateTime(t0(5))
	if !closerTo(target, candidateCloser, current) {
		t.Fatal("expected the closer candidate to win")
	}

	candidateFarther := db.DateTime(t0(0))
	currentCloser := db.DateTime(t0(9))
	if closerTo(target, candidateFarther, currentCloser) {
		t.Fatal("expected the farther candidate to lose")
	}

	// Exact tie: candidate and current are equidistant - closerTo must return true (<=), matching
	// the "advance on tie" behavior nearestLocation relies on.
	tie := db.DateTime(t0(10))
	if !closerTo(target, tie, tie) {
		t.Fatal("expected a tie to satisfy closerTo (<=)")
	}
}

// TestAbsDuration covers both the negative and non-negative branches.
func TestAbsDuration(t *testing.T) {
	if got := absDuration(-5 * time.Second); got != 5*time.Second {
		t.Fatalf("absDuration(-5s) = %v, want 5s", got)
	}
	if got := absDuration(5 * time.Second); got != 5*time.Second {
		t.Fatalf("absDuration(5s) = %v, want 5s", got)
	}
	if got := absDuration(0); got != 0 {
		t.Fatalf("absDuration(0) = %v, want 0", got)
	}
}

// TestSafetyCarFromScope_PrismaAdapter mirrors usecases.TestSafetyCarFromScope for this package's
// intentionally-duplicated copy (see race_stream.repository.go's doc comment on why it isn't
// shared) - proving both copies agree on behavior.
func TestSafetyCarFromScope_PrismaAdapter(t *testing.T) {
	if got := safetyCarFromScope("Virtual", ""); got == nil || *got != "VSC" {
		t.Fatalf("expected VSC for a virtual scope, got %v", got)
	}
	if got := safetyCarFromScope("Safety Car", ""); got == nil || *got != "SC" {
		t.Fatalf("expected SC for \"Safety Car\", got %v", got)
	}
	if got := safetyCarFromScope("", "SafetyCar"); got == nil || *got != "SC" {
		t.Fatalf("expected SC for \"SafetyCar\" (no space), got %v", got)
	}
	if got := safetyCarFromScope("Track", "Flag"); got != nil {
		t.Fatalf("expected nil for a plain track/flag scope, got %v", *got)
	}
}
