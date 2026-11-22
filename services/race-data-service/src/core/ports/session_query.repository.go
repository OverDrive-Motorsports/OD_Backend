/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_query.repository.go - Package ports source file for services/race-data-service/src/core/ports.
##
*/

package ports

import (
	"context"

	"overdrive/services/race-data-service/src/core/domain"
)

type SessionQueryRepository interface {
	// GetSessionDataset and GetDriverDataset read page.Fetch() rows from page.Offset.
	GetSessionDataset(ctx context.Context, sessionID string, dataset string, page domain.Page) ([]map[string]any, error)
	GetDriverDataset(ctx context.Context, sessionID string, driverNumber int, dataset string, page domain.Page) ([]map[string]any, error)
	GetDriverLapLocation(ctx context.Context, sessionID string, driverNumber int, lapNumber int) (map[string]any, error)
	GetDriverBroadcast(ctx context.Context, sessionID string, driverNumber int) (string, error)
}
