/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## accept_ingestion_batch.usecase.go - Package usecases source file for services/race-data-service/src/core/usecases.
	##
*/

package usecases

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
	contracts "overdrive/shared/contracts/ingestion"
)

type AcceptIngestionBatchUseCase struct {
	serviceName string
	repository  ports.RaceDataIngestionRepository
	broadcaster ports.RaceControlBroadcaster
	now         func() time.Time
}

// NewAcceptIngestionBatchUseCase builds and returns a accept ingestion batch use case with its required dependencies.
// broadcaster may be nil (e.g. in tests) — publishing is then skipped.
func NewAcceptIngestionBatchUseCase(serviceName string, repository ports.RaceDataIngestionRepository, broadcaster ports.RaceControlBroadcaster) *AcceptIngestionBatchUseCase {
	return &AcceptIngestionBatchUseCase{
		serviceName: serviceName,
		repository:  repository,
		broadcaster: broadcaster,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// Execute runs the use case workflow and returns the resulting domain payload.
func (u *AcceptIngestionBatchUseCase) Execute(ctx context.Context, batch contracts.Batch) (domain.IngestionBatchAck, error) {
	if batch.BatchID == "" {
		return domain.IngestionBatchAck{}, fmt.Errorf("batch_id is required")
	}
	if len(batch.Datasets) == 0 {
		return domain.IngestionBatchAck{}, fmt.Errorf("at least one dataset is required")
	}
	for _, dataset := range batch.Datasets {
		if dataset.TargetService != u.serviceName {
			return domain.IngestionBatchAck{}, fmt.Errorf("dataset %s targets %s, not %s", dataset.Name, dataset.TargetService, u.serviceName)
		}
	}
	if err := u.repository.StoreBatch(ctx, batch); err != nil {
		return domain.IngestionBatchAck{}, err
	}
	u.publishRaceControlEvents(batch)
	return domain.NewIngestionBatchAck(u.serviceName, batch, u.now()), nil
}

// publishRaceControlEvents notifies any long-polling GET .../race/control waiters
// once a race_control dataset has been durably stored. Best-effort: publishing
// never fails the ingestion request.
func (u *AcceptIngestionBatchUseCase) publishRaceControlEvents(batch contracts.Batch) {
	if u.broadcaster == nil {
		return
	}
	sessionID := contracts.SessionID(batch.Provider, batch.Context.SessionKey)
	for _, dataset := range batch.Datasets {
		if dataset.Name != "race_control" {
			continue
		}
		events := make([]domain.RaceControlEvent, 0, len(dataset.Rows))
		for _, row := range dataset.Rows {
			events = append(events, mapRaceControlEvent(row))
		}
		u.broadcaster.Publish(sessionID, events)
	}
}

// mapRaceControlEvent converts a raw ingestion row into a public RaceControlEvent.
func mapRaceControlEvent(row map[string]any) domain.RaceControlEvent {
	event := domain.RaceControlEvent{
		Category:  stringifyAny(row["category"]),
		Flag:      stringifyAny(row["flag"]),
		Message:   stringifyAny(row["message"]),
		LapNumber: intValueFromRow(row["lap_number"]),
	}
	if ts, ok := parseTimeAny(row["date_utc"]); ok {
		event.Timestamp = ts
	}
	event.SafetyCar = safetyCarFromScope(stringifyAny(row["scope"]), stringifyAny(row["category"]))
	return event
}

// safetyCarFromScope derives the best-effort safetyCar contract value from the
// available OpenF1 category/scope fields. There is no dedicated safety-car
// field upstream, so this is a heuristic — see doc/endpoint.md gap note.
func safetyCarFromScope(scope string, category string) *string {
	combined := strings.ToLower(scope + " " + category)
	switch {
	case strings.Contains(combined, "virtual"):
		value := "VSC"
		return &value
	case strings.Contains(combined, "safety car") || strings.Contains(combined, "safetycar"):
		value := "SC"
		return &value
	default:
		return nil
	}
}

// stringifyAny renders a dynamic ingestion payload value as a display string.
func stringifyAny(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// intValueFromRow converts common numeric payload values into an int.
func intValueFromRow(value any) int {
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
		parsed, err := strconv.Atoi(v)
		if err == nil {
			return parsed
		}
	}
	return 0
}

// parseTimeAny parses a dynamic ingestion payload timestamp value.
func parseTimeAny(value any) (time.Time, bool) {
	text := stringifyAny(value)
	if text == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}
