/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_feed.service.go - Use case port for reading/replacing a session's video feed list.
##
*/

package ports

import (
	"context"

	"overdrive/services/championship-service/src/core/domain"
)

// SessionFeedUseCase backs GET/PUT /sessions/{sessionId}/broadcast. Both methods return
// (nil, nil) for an unknown session; ReplaceSessionFeeds returns domain.ErrInvalidFeed
// (wrapped) when the list violates domain.ValidateFeeds.
type SessionFeedUseCase interface {
	GetSessionFeeds(ctx context.Context, sessionID string) (*domain.SessionBroadcast, error)
	ReplaceSessionFeeds(ctx context.Context, sessionID string, feeds []domain.Feed) (*domain.SessionBroadcast, error)
}
