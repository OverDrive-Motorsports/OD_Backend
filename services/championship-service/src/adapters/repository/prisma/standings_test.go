/**
##
## OverDrive 2026
## All Technical rights reserved
##
## standings_test.go - Unit tests for result status derivation, standing mapping/ordering and headshot URLs.
##
*/

package prismaadapter

import (
	"testing"

	db "overdrive/services/championship-service/resources/db"
	"overdrive/services/championship-service/src/core/domain"
)

// TestResultStatus proves the flag precedence (dsq > dns > dnf), the "finished" fallback for a
// row with a position, the unknown ("") fallback without position or flags, and that only a
// real boolean true counts as a flag.
func TestResultStatus(t *testing.T) {
	cases := map[string]struct {
		row  map[string]any
		want string
	}{
		"classified finisher":        {map[string]any{"position": float64(3), "dnf": false, "dns": false, "dsq": false}, domain.ResultStatusFinished},
		"dnf":                        {map[string]any{"dnf": true}, domain.ResultStatusDNF},
		"classified dnf keeps dnf":   {map[string]any{"position": float64(17), "dnf": true}, domain.ResultStatusDNF},
		"dns":                        {map[string]any{"dns": true}, domain.ResultStatusDNS},
		"dsq wins over dnf":          {map[string]any{"dnf": true, "dsq": true}, domain.ResultStatusDSQ},
		"dns wins over dnf":          {map[string]any{"dnf": true, "dns": true}, domain.ResultStatusDNS},
		"no flags no position":       {map[string]any{}, ""},
		"string flag is not a flag":  {map[string]any{"dnf": "true"}, ""},
		"null position, flags false": {map[string]any{"position": nil, "dnf": false}, ""},
	}
	for name, tc := range cases {
		if got := resultStatus(tc.row); got != tc.want {
			t.Fatalf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

// TestMapStandingRow proves a NULL position stays nil (never 0), the stored status column wins,
// and a row stored before the status column was populated derives it from the raw payload.
func TestMapStandingRow(t *testing.T) {
	position := 2
	stored := "dsq"

	row := &db.SessionResultRowModel{InnerSessionResultRow: db.InnerSessionResultRow{
		DriverNumber: 44, Position: &position, Status: &stored,
		Raw: db.JSON(`{"gap_to_leader": 1.5, "dnf": true}`),
	}}
	got := mapStandingRow(row, "t1")
	if got.Position == nil || *got.Position != 2 || got.Status != "dsq" || got.TeamID != "t1" || got.GapToLeader != "1.5" {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	legacy := &db.SessionResultRowModel{InnerSessionResultRow: db.InnerSessionResultRow{
		DriverNumber: 16, Raw: db.JSON(`{"position": null, "dnf": true}`),
	}}
	got = mapStandingRow(legacy, "")
	if got.Position != nil || got.Status != domain.ResultStatusDNF {
		t.Fatalf("expected nil position and status derived from raw, got %+v", got)
	}
}

// TestSortStandings proves classified drivers come first by position, then non-classified ones
// (nil position) ordered by driver number.
func TestSortStandings(t *testing.T) {
	one, two := 1, 2
	items := []domain.StandingRow{
		{DriverNumber: 81},
		{Position: &two, DriverNumber: 4},
		{DriverNumber: 16},
		{Position: &one, DriverNumber: 44},
	}
	sortStandings(items)
	want := []int{44, 4, 16, 81}
	for index, driver := range want {
		if items[index].DriverNumber != driver {
			t.Fatalf("position %d: got driver %d, want %d (order %+v)", index, items[index].DriverNumber, driver, items)
		}
	}
}

// TestHeadshotURLPtr proves only absolute http(s) URLs within the column size are kept, trimmed;
// anything else is dropped so it never fails ingestion nor overwrites a stored picture.
func TestHeadshotURLPtr(t *testing.T) {
	valid := "https://media.formula1.com/d_driver_fallback_image.png/content/dam/fom-website/drivers/L/LEWHAM01.png"
	if got := headshotURLPtr("  " + valid + " "); got == nil || *got != valid {
		t.Fatalf("expected the trimmed URL, got %v", got)
	}
	long := "https://a.b/" + string(make([]byte, maxHeadshotURLLength))
	for name, value := range map[string]any{
		"nil":          nil,
		"empty":        "",
		"relative":     "/drivers/LEWHAM01.png",
		"other scheme": "javascript:alert(1)",
		"ftp":          "ftp://a.b/x.png",
		"too long":     long,
		"not a string": 42,
		"missing host": "https:///x.png",
	} {
		if got := headshotURLPtr(value); got != nil {
			t.Fatalf("%s: expected nil, got %q", name, *got)
		}
	}
}
