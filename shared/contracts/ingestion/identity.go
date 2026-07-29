/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## identity.go - Package ingestion source file for shared/contracts/ingestion.
	##
*/

package ingestion

import (
	"fmt"
	"strings"
)

// ProviderID builds the stable internal identifier for the related ingestion entity.
func ProviderID(provider string) string {
	return fmt.Sprintf("provider:%s", sanitize(provider))
}

// ChampionshipID builds the stable internal identifier for the related ingestion entity.
func ChampionshipID(provider string, championshipCode string) string {
	return fmt.Sprintf("%s:championship:%s", sanitize(provider), sanitize(championshipCode))
}

// EventID builds the stable internal identifier for the related ingestion entity.
func EventID(provider string, meetingKey int) string {
	return fmt.Sprintf("%s:event:%d", sanitize(provider), meetingKey)
}

// SessionID builds the stable internal identifier for the related ingestion entity.
func SessionID(provider string, sessionKey int) string {
	return fmt.Sprintf("%s:session:%d", sanitize(provider), sessionKey)
}

// TeamID builds the stable internal identifier for the related ingestion entity.
func TeamID(provider string, championshipCode string, teamExternalKey string) string {
	return fmt.Sprintf("%s:%s:team:%s", sanitize(provider), sanitize(championshipCode), sanitize(teamExternalKey))
}

// DriverID builds the stable internal identifier for the related ingestion entity.
func DriverID(provider string, championshipCode string, driverExternalKey string) string {
	return fmt.Sprintf("%s:%s:driver:%s", sanitize(provider), sanitize(championshipCode), sanitize(driverExternalKey))
}

// sanitize normalizes identifier fragments so they can be safely used in generated IDs.
func sanitize(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "/", "-")
	if value == "" {
		return "unknown"
	}
	return value
}
