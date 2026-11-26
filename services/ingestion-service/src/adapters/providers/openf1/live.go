/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live.go - OpenF1 calls of the live jobs: session lookup and time-filtered rows.
##
*/

package openf1

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
)

// openF1TimeLayout is the filter form accepted by OpenF1: UTC, without offset.
const openF1TimeLayout = "2006-01-02T15:04:05.000"

// FindSession returns the session, or domain.ErrLiveSessionNotFound when OpenF1 does not know it
// or gives no usable dates.
func (c *Client) FindSession(ctx context.Context, sessionKey int) (domain.LiveSession, error) {
	query := url.Values{}
	query.Set("session_key", strconv.Itoa(sessionKey))
	rows, err := c.fetchWithRetry(ctx, fmt.Sprintf("%s/sessions?%s", c.baseURL, query.Encode()))
	if isNotFound(err) || (err == nil && len(rows) == 0) {
		return domain.LiveSession{}, domain.ErrLiveSessionNotFound
	}
	if err != nil {
		return domain.LiveSession{}, err
	}
	start, startErr := time.Parse(time.RFC3339, asString(rows[0]["date_start"]))
	end, endErr := time.Parse(time.RFC3339, asString(rows[0]["date_end"]))
	meetingKey := intValue(rows[0]["meeting_key"])
	if startErr != nil || endErr != nil || meetingKey <= 0 {
		return domain.LiveSession{}, fmt.Errorf("%w: incomplete session", domain.ErrLiveSessionNotFound)
	}
	return domain.LiveSession{SessionKey: sessionKey, MeetingKey: meetingKey, StartUTC: start.UTC(), EndUTC: end.UTC()}, nil
}

// FetchWindow returns the rows of the resource dated within the window (bounds included). OpenF1
// reads the comparison from the parameter name: "date>" gives "date>=<value>".
func (c *Client) FetchWindow(ctx context.Context, resource domain.LiveResource, sessionKey int, window domain.LiveWindow) ([]map[string]any, error) {
	query := url.Values{}
	query.Set("session_key", strconv.Itoa(sessionKey))
	if resource.DateField != "" && window.From != nil {
		query.Set(resource.DateField+">", window.From.UTC().Format(openF1TimeLayout))
	}
	if resource.DateField != "" && window.To != nil {
		query.Set(resource.DateField+"<", window.To.UTC().Format(openF1TimeLayout))
	}
	rows, err := c.fetchWithRetry(ctx, fmt.Sprintf("%s/%s?%s", c.baseURL, resource.Name, query.Encode()))
	if isNotFound(err) {
		return []map[string]any{}, nil
	}
	return rows, err
}
