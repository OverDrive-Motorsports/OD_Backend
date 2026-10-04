/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_feed.repository.go - Prisma-backed read/replace of the Session.feeds Json column.
##
*/

package prismaadapter

import (
	"context"
	"time"

	db "overdrive/services/championship-service/resources/db"
	"overdrive/services/championship-service/src/core/domain"
)

type SessionFeedRepository struct {
	client *db.PrismaClient
}

// NewSessionFeedRepository builds and returns a session feed repository with its required dependencies.
func NewSessionFeedRepository(client *db.PrismaClient) *SessionFeedRepository {
	return &SessionFeedRepository{client: client}
}

// GetSessionFeeds loads the feed list of one session; (nil, nil) when the session is unknown.
func (r *SessionFeedRepository) GetSessionFeeds(ctx context.Context, sessionID string) (*domain.SessionBroadcast, error) {
	row, err := r.client.Session.FindUnique(db.Session.ID.Equals(sessionID)).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return &domain.SessionBroadcast{SessionID: row.ID, Feeds: decodeFeeds(row.Feeds)}, nil
}

// ReplaceSessionFeeds overwrites the whole feed list of one session and returns the stored
// result; (nil, nil) when the session is unknown (Prisma's updateOne raises ErrNotFound).
func (r *SessionFeedRepository) ReplaceSessionFeeds(ctx context.Context, sessionID string, feeds []domain.Feed) (*domain.SessionBroadcast, error) {
	payload, err := encodeFeeds(feeds)
	if err != nil {
		return nil, err
	}
	row, err := r.client.Session.FindUnique(db.Session.ID.Equals(sessionID)).Update(
		db.Session.Feeds.Set(payload),
	).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return &domain.SessionBroadcast{SessionID: row.ID, Feeds: decodeFeeds(row.Feeds)}, nil
}

// GetSessionWindow loads the start/end bounds of one session, used to check feed start times;
// (nil, nil) when the session is unknown.
func (r *SessionFeedRepository) GetSessionWindow(ctx context.Context, sessionID string) (*domain.SessionWindow, error) {
	row, err := r.client.Session.FindUnique(db.Session.ID.Equals(sessionID)).Exec(ctx)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	window := &domain.SessionWindow{Start: time.Time(row.StartedAtUtc)}
	if endedAt, ok := row.EndedAtUtc(); ok {
		end := time.Time(endedAt)
		window.End = &end
	}
	return window, nil
}
