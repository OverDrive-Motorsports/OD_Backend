/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live.service.go - Live ingestion ports: OpenF1 live provider and live use case.
##
*/

package ports

import (
	"context"

	"overdrive/services/ingestion-service/src/core/domain"
)

// OpenF1LiveProvider reads OpenF1 for a live job.
type OpenF1LiveProvider interface {
	// FindSession returns the session, or domain.ErrLiveSessionNotFound.
	FindSession(ctx context.Context, sessionKey int) (domain.LiveSession, error)
	// FetchWindow returns the rows of the resource within the window, none when there are none.
	FetchWindow(ctx context.Context, resource domain.LiveResource, sessionKey int, window domain.LiveWindow) ([]map[string]any, error)
}

type LiveIngestionUseCase interface {
	// Start starts the live job of the session; started is false when it was already running.
	Start(ctx context.Context, sessionKey int) (status domain.LiveStatus, started bool, err error)
	Stop(ctx context.Context, sessionKey int) (domain.LiveStatus, error)
	Status(sessionKey int) (domain.LiveStatus, error)
}
