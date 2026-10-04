/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_feed.repository.go - Persistence port for a session's global video feed list.
##
*/

package ports

import (
	"context"

	"overdrive/services/championship-service/src/core/domain"
)

// SessionFeedRepository reads and replaces the feed list stored on a Session row.
// Every method returns (nil, nil) when the session does not exist.
type SessionFeedRepository interface {
	GetSessionFeeds(ctx context.Context, sessionID string) (*domain.SessionBroadcast, error)
	ReplaceSessionFeeds(ctx context.Context, sessionID string, feeds []domain.Feed) (*domain.SessionBroadcast, error)
	GetSessionWindow(ctx context.Context, sessionID string) (*domain.SessionWindow, error)
}
