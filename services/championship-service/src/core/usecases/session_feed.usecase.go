/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_feed.usecase.go - Validates and persists a session's global video feed list.
##
*/

package usecases

import (
	"context"

	"overdrive/services/championship-service/src/core/domain"
	"overdrive/services/championship-service/src/core/ports"
)

type SessionFeedUseCase struct {
	repository ports.SessionFeedRepository
}

// NewSessionFeedUseCase builds and returns a session feed use case with its required dependencies.
func NewSessionFeedUseCase(repository ports.SessionFeedRepository) *SessionFeedUseCase {
	return &SessionFeedUseCase{repository: repository}
}

// GetSessionFeeds returns the session's feed list, or nil when the session is unknown.
func (u *SessionFeedUseCase) GetSessionFeeds(ctx context.Context, sessionID string) (*domain.SessionBroadcast, error) {
	return u.repository.GetSessionFeeds(ctx, sessionID)
}

// ReplaceSessionFeeds validates the supplied list with domain.ValidateFeeds and, only when it
// passes, replaces the whole stored list. When at least one feed carries a startedAtUtc, the
// session window is loaded first (unknown session → (nil, nil), i.e. 404) and every start time
// is checked against it. Start times are stored in UTC and a nil list becomes an empty one,
// so the column always holds a JSON array.
func (u *SessionFeedUseCase) ReplaceSessionFeeds(ctx context.Context, sessionID string, feeds []domain.Feed) (*domain.SessionBroadcast, error) {
	if err := domain.ValidateFeeds(feeds); err != nil {
		return nil, err
	}
	if domain.HasFeedStartTimes(feeds) {
		window, err := u.repository.GetSessionWindow(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		if window == nil {
			return nil, nil
		}
		if err := domain.ValidateFeedStartTimes(feeds, *window); err != nil {
			return nil, err
		}
	}
	return u.repository.ReplaceSessionFeeds(ctx, sessionID, domain.NormalizeFeeds(feeds))
}
