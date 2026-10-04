/**
##
## OverDrive 2026
## All Technical rights reserved
##
## standings.go - Session result status derivation and standing row mapping/ordering.
##
*/

package prismaadapter

import (
	"sort"

	db "overdrive/services/championship-service/resources/db"
	"overdrive/services/championship-service/src/core/domain"
)

// resultStatus derives the finishing status of a session result row from the
// provider's dsq / dns / dnf flags (most severe first). Without any of those
// flags it falls back to "finished" when the row has a position, and to ""
// (unknown) otherwise. It reads both the mapped ingestion row and the stored
// raw OpenF1 row, which carry the flags under the same keys.
func resultStatus(row map[string]any) string {
	switch {
	case truthy(row["dsq"]):
		return domain.ResultStatusDSQ
	case truthy(row["dns"]):
		return domain.ResultStatusDNS
	case truthy(row["dnf"]):
		return domain.ResultStatusDNF
	}
	if intPtr(row["position"]) != nil {
		return domain.ResultStatusFinished
	}
	return ""
}

// truthy reports whether a JSON-decoded flag is a true boolean.
func truthy(value any) bool {
	flag, ok := value.(bool)
	return ok && flag
}

// mapStandingRow maps one stored session result row into the API standing row. The status
// column wins; rows ingested before it was populated fall back to deriving it from the raw
// provider payload, so existing data is fixed without a re-ingestion. A missing position
// stays nil instead of collapsing to 0.
func mapStandingRow(row *db.SessionResultRowModel, teamID string) domain.StandingRow {
	raw := rawMap(row.Raw)
	status, ok := row.Status()
	if !ok || status == "" {
		status = resultStatus(raw)
	}
	var position *int
	if value, ok := row.Position(); ok {
		position = &value
	}
	points, _ := row.Points()
	return domain.StandingRow{
		Position:     position,
		DriverNumber: row.DriverNumber,
		TeamID:       teamID,
		Status:       status,
		GapToLeader:  stringifyAny(raw["gap_to_leader"]),
		Points:       points,
	}
}

// sortStandings orders classified drivers by position, then non-classified drivers (nil
// position) after them by driver number, so the order is deterministic.
func sortStandings(items []domain.StandingRow) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i].Position, items[j].Position
		switch {
		case left != nil && right != nil:
			return *left < *right
		case left != nil:
			return true
		case right != nil:
			return false
		default:
			return items[i].DriverNumber < items[j].DriverNumber
		}
	})
}
