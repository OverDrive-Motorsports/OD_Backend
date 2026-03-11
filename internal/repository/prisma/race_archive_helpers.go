/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_helpers.go - Shared helper functions for archive inference, JSON mapping, and provider row parsing.
##
*/

package prisma

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"overdrive/internal/domain"
	"overdrive/resources/db"
)

func inferEventBounds(archive domain.RaceArchive) (time.Time, time.Time) {
	var start time.Time
	var end time.Time

	for _, row := range archive.AllSessions {
		sessionStart := firstTimeFromRows(row, "date_start")
		sessionEnd := firstTimeFromRows(row, "date_end")

		if !sessionStart.IsZero() && (start.IsZero() || sessionStart.Before(start)) {
			start = sessionStart
		}
		if !sessionEnd.IsZero() && sessionEnd.After(end) {
			end = sessionEnd
		}
	}

	if start.IsZero() {
		start = firstTimeFromRows(archive.RaceSession, "date_start")
	}
	if end.IsZero() {
		end = firstTimeFromRows(archive.RaceSession, "date_end")
	}

	if start.IsZero() {
		start = time.Now().UTC()
	}
	if end.IsZero() {
		end = start.Add(2 * time.Hour)
	}
	if end.Before(start) {
		end = start
	}

	return start, end
}

// resolveArchiveMode maps metadata scope to the persisted archive mode enum.
func resolveArchiveMode(driverNumber int) db.ArchiveMode {
	if driverNumber > 0 {
		return db.ArchiveModeDriverFocused
	}
	return db.ArchiveModeFull
}

// resolveEventStatus infers event status from start/end bounds and current time.
func resolveEventStatus(start, end time.Time) db.EventStatus {
	now := time.Now().UTC()
	if !end.IsZero() && now.After(end) {
		return db.EventStatusFinished
	}
	if !start.IsZero() && now.After(start) {
		return db.EventStatusLive
	}
	return db.EventStatusScheduled
}

// resolveSessionType maps provider session labels to internal enum values.
func resolveSessionType(sessionName string) db.SessionType {
	name := strings.ToLower(strings.TrimSpace(sessionName))
	switch {
	case strings.Contains(name, "sprint"):
		return db.SessionTypeSprint
	case strings.Contains(name, "qual"):
		return db.SessionTypeQuali
	case strings.Contains(name, "practice"), strings.Contains(name, "fp"):
		return db.SessionTypePractice
	default:
		return db.SessionTypeRace
	}
}

// resolveSessionStatus infers session status from start/end bounds and current time.
func resolveSessionStatus(start, end time.Time) db.SessionStatus {
	now := time.Now().UTC()
	if !end.IsZero() && now.After(end) {
		return db.SessionStatusFinished
	}
	if !start.IsZero() && now.After(start) {
		return db.SessionStatusLive
	}
	return db.SessionStatusScheduled
}

// sessionBroadcastURL builds the placeholder broadcast URL stored on the session row.
func sessionBroadcastURL(sessionKey int) string {
	if sessionKey > 0 {
		return fmt.Sprintf("https://www.youtube.com/watch?v=dQw4w9WgXcQ&session=%d", sessionKey)
	}
	return "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
}

// sessionDriverBroadcastURL builds the placeholder broadcast URL stored per driver for one session.
func sessionDriverBroadcastURL(sessionKey, driverNumber int) string {
	base := sessionBroadcastURL(sessionKey)
	if driverNumber > 0 {
		return fmt.Sprintf("%s&driver=%d", base, driverNumber)
	}
	return base
}

// marshalPrismaJSON converts any Go value to prisma JSON payload format.
func marshalPrismaJSON(value any) (db.JSON, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return db.JSON(raw), nil
}

// unmarshalPrismaJSON decodes prisma JSON payloads into Go values.
func unmarshalPrismaJSON(raw db.JSON, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal([]byte(raw), out)
}

// unmarshalMetadataEnvelope decodes metadata in both envelope and legacy metadata-only formats.
func unmarshalMetadataEnvelope(raw db.JSON) (metadataEnvelope, error) {
	out := metadataEnvelope{}
	if err := unmarshalPrismaJSON(raw, &out); err != nil {
		return metadataEnvelope{}, err
	}

	// Legacy fallback when only Metadata was persisted in the JSON field.
	if out.Metadata.Provider == "" && out.Metadata.MeetingKey == 0 && out.Metadata.RaceSessKey == 0 &&
		len(out.AllSessions) == 0 && len(out.Meeting) == 0 && len(out.RaceSession) == 0 {
		legacy := domain.Metadata{}
		if err := unmarshalPrismaJSON(raw, &legacy); err != nil {
			return metadataEnvelope{}, err
		}
		out.Metadata = legacy
	}

	return out, nil
}

// firstTimeFromRows extracts a timestamp field from a provider row map.
func firstTimeFromRows(row map[string]any, key string) time.Time {
	if row == nil {
		return time.Time{}
	}
	raw, ok := row[key]
	if !ok || raw == nil {
		return time.Time{}
	}
	switch v := raw.(type) {
	case time.Time:
		return v.UTC()
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(v))
		if err != nil {
			return time.Time{}
		}
		return parsed.UTC()
	default:
		return time.Time{}
	}
}

// readOptionalStringField reads a non-empty string map value as an optional pointer.
func readOptionalStringField(row map[string]any, key string) *string {
	if row == nil {
		return nil
	}
	raw, ok := row[key]
	if !ok || raw == nil {
		return nil
	}
	s, ok := raw.(string)
	if !ok {
		return nil
	}
	return optionalString(s)
}

// readOptionalIntField reads an optional integer map value as a pointer.
func readOptionalIntField(row map[string]any, key string) *int {
	if row == nil {
		return nil
	}
	raw, ok := row[key]
	if !ok || raw == nil {
		return nil
	}

	switch value := raw.(type) {
	case int:
		return &value
	case int64:
		parsed := int(value)
		return &parsed
	case float64:
		parsed := int(value)
		return &parsed
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil
		}
		return &parsed
	default:
		return nil
	}
}

// fallbackString returns fallback when the input string is blank.
func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// nonNilMap guarantees a non-nil map for JSON serialization and handlers.
func nonNilMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

// nonNilRows guarantees a non-nil rows slice for JSON serialization and handlers.
func nonNilRows(in []map[string]any) []map[string]any {
	if in == nil {
		return []map[string]any{}
	}
	return in
}

// optionalString returns a pointer to non-empty strings.
func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// optionalInt returns a pointer when the value is strictly positive.
func optionalInt(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

// optionalTime returns a pointer when the timestamp is non-zero.
func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

// optionalStringValue unwraps optional strings into plain values for DTOs.
func optionalStringValue(value string) string {
	return strings.TrimSpace(value)
}

// optionalIntPointer copies an optional integer into a pointer for DTOs.
func optionalIntPointer(value int) *int {
	if value == 0 {
		return nil
	}
	copy := value
	return &copy
}

// cloneMap copies a provider row map for safe reuse in merged payloads.
func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

// mergeUniqueRows appends rows once using their JSON representation as a stable dedupe key.
func mergeUniqueRows(existing []map[string]any, incoming []map[string]any, seen map[string]struct{}) []map[string]any {
	if seen == nil {
		seen = map[string]struct{}{}
	}

	for _, row := range incoming {
		key := rowSignature(row)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		existing = append(existing, cloneMap(row))
	}

	return existing
}

// rowSignature produces a deterministic dedupe key for one provider row.
func rowSignature(row map[string]any) string {
	raw, err := json.Marshal(row)
	if err != nil {
		return fmt.Sprint(row)
	}
	return string(raw)
}

// readIntField reads a numeric provider field and returns zero when missing.
func readIntField(row map[string]any, key string) int {
	if row == nil {
		return 0
	}

	raw, ok := row[key]
	if !ok || raw == nil {
		return 0
	}

	switch value := raw.(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
		if parsed, err := value.Float64(); err == nil {
			return int(parsed)
		}
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return 0
		}
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return int(parsed)
		}
	}

	return 0
}

// readStringField reads a provider field and normalizes it to a trimmed string.
func readStringField(row map[string]any, key string) string {
	if row == nil {
		return ""
	}

	raw, ok := row[key]
	if !ok || raw == nil {
		return ""
	}

	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	case json.Number:
		return strings.TrimSpace(value.String())
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case int:
		return strconv.Itoa(value)
	case int8:
		return strconv.FormatInt(int64(value), 10)
	case int16:
		return strconv.FormatInt(int64(value), 10)
	case int32:
		return strconv.FormatInt(int64(value), 10)
	case int64:
		return strconv.FormatInt(value, 10)
	case bool:
		return strconv.FormatBool(value)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

// readFirstStringField returns the first non-empty provider string among several keys.
func readFirstStringField(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := readStringField(row, key); value != "" {
			return value
		}
	}
	return ""
}

// teamExternalKey builds a stable team key from its provider label.
func teamExternalKey(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return "unknown"
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	key := strings.Trim(builder.String(), "-")
	if key == "" {
		return "unknown"
	}
	return key
}

// normalizeColor normalizes provider team colors to #RRGGBB when possible.
func normalizeColor(value string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "#"))
	if trimmed == "" {
		return ""
	}

	if len(trimmed) == 3 {
		trimmed = strings.Repeat(string(trimmed[0]), 2) +
			strings.Repeat(string(trimmed[1]), 2) +
			strings.Repeat(string(trimmed[2]), 2)
	}

	if len(trimmed) != 6 {
		return ""
	}

	for _, r := range trimmed {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return ""
		}
	}

	return "#" + strings.ToUpper(trimmed)
}

// chunkRows splits raw datasets into deterministic chunks for archive storage.
func chunkRows(rows []map[string]any, chunkSize int) [][]map[string]any {
	if len(rows) == 0 {
		return [][]map[string]any{{}}
	}
	if chunkSize <= 0 || len(rows) <= chunkSize {
		return [][]map[string]any{rows}
	}

	chunks := make([][]map[string]any, 0, (len(rows)+chunkSize-1)/chunkSize)
	for start := 0; start < len(rows); start += chunkSize {
		end := start + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		chunks = append(chunks, rows[start:end])
	}

	return chunks
}

// newUUID generates a UUIDv4 string without pulling a new dependency.
func newUUID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16]), nil
}
