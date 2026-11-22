/**
##
## OverDrive 2026
## All Technical rights reserved
##
## client.go - Package openf1 source file for services/ingestion-service/src/adapters/providers/openf1.
##
*/

package openf1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
	"overdrive/shared/bootstrap"
)

// DefaultMaxResponseBytes bounds one OpenF1 response. Heavy resources (car_data, location) are
// fetched one driver at a time, a few megabytes each; a whole session in one response is
// several hundred megabytes and is refused instead of being decoded.
const DefaultMaxResponseBytes int64 = 128 << 20

// maxRateLimitWaits bounds how many 429 answers one request waits through, on top of the normal
// retries, and maxRetryAfter how long a single Retry-After may make it wait.
const (
	maxRateLimitWaits = 5
	maxRetryAfter     = 60 * time.Second
)

type Client struct {
	baseURL          string
	httpClient       *http.Client
	retries          int
	retryDelay       time.Duration
	minInterval      time.Duration
	maxResponseBytes int64

	paceMu   sync.Mutex
	lastCall time.Time
}

// ClientOptions tunes the pacing and limits of the OpenF1 client.
type ClientOptions struct {
	// MinInterval is the minimum time between two OpenF1 requests (rate limit courtesy).
	MinInterval time.Duration
	// MaxResponseBytes bounds one response; zero means DefaultMaxResponseBytes.
	MaxResponseBytes int64
}

// NewClient builds and returns a client with its required dependencies.
func NewClient(baseURL string, timeout time.Duration, retries int, retryDelay time.Duration, options ClientOptions) *Client {
	maxResponseBytes := options.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = DefaultMaxResponseBytes
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retries:          retries,
		retryDelay:       retryDelay,
		minInterval:      options.MinInterval,
		maxResponseBytes: maxResponseBytes,
	}
}

// upstreamError is a non-2xx OpenF1 answer. RetryAfter is set for a 429 carrying the header.
type upstreamError struct {
	status     int
	body       string
	retryAfter time.Duration
}

func (e *upstreamError) Error() string {
	if e.status >= http.StatusInternalServerError {
		return fmt.Sprintf("upstream status %d: %s", e.status, e.body)
	}
	return fmt.Sprintf("upstream request failed with status %d: %s", e.status, e.body)
}

// retryable reports whether another attempt may succeed: server errors, timeouts and rate
// limiting. Any other 4xx (unknown resource, no data) fails at once.
func (e *upstreamError) retryable() bool {
	return e.status >= http.StatusInternalServerError || e.status == http.StatusTooManyRequests || e.status == http.StatusRequestTimeout
}

// isNotFound reports whether err is an OpenF1 404.
func isNotFound(err error) bool {
	var upstream *upstreamError
	return errors.As(err, &upstream) && upstream.status == http.StatusNotFound
}

// Fetch retrieves provider data for the requested resource and ingestion context. OpenF1
// answers 404 "No results found" for an empty result: it is returned as no rows, not an error.
func (c *Client) Fetch(ctx context.Context, resource string, request domain.OpenF1IngestionRequest) ([]map[string]any, error) {
	if resource == "starting_grid" {
		return c.fetchStartingGrid(ctx, request)
	}

	endpoint, err := c.buildURL(resource, request)
	if err != nil {
		return nil, err
	}
	rows, err := c.fetchWithRetry(ctx, endpoint)
	if isNotFound(err) {
		return []map[string]any{}, nil
	}
	return rows, err
}

// ListDriverNumbers returns the driver numbers of the session, sorted, used to fetch the heavy
// resources one driver at a time.
func (c *Client) ListDriverNumbers(ctx context.Context, request domain.OpenF1IngestionRequest) ([]int, error) {
	query := url.Values{}
	query.Set("session_key", strconv.Itoa(request.SessionKey))
	rows, err := c.fetchWithRetry(ctx, fmt.Sprintf("%s/drivers?%s", c.baseURL, query.Encode()))
	if isNotFound(err) {
		return []int{}, nil
	}
	if err != nil {
		return nil, err
	}
	seen := map[int]struct{}{}
	drivers := make([]int, 0, len(rows))
	for _, row := range rows {
		number := intValue(row["driver_number"])
		if number <= 0 {
			continue
		}
		if _, done := seen[number]; done {
			continue
		}
		seen[number] = struct{}{}
		drivers = append(drivers, number)
	}
	sort.Ints(drivers)
	return drivers, nil
}

// fetchStartingGrid retrieves the grid, falling back from race sessions to their qualifying session.
func (c *Client) fetchStartingGrid(ctx context.Context, request domain.OpenF1IngestionRequest) ([]map[string]any, error) {
	endpoint, err := c.buildURL("starting_grid", request)
	if err != nil {
		return nil, err
	}

	rows, err := c.fetchWithRetry(ctx, endpoint)
	if err == nil {
		return rows, nil
	}
	if !isNotFound(err) {
		return nil, err
	}

	fallbackSessionKey, fallbackErr := c.findStartingGridSession(ctx, request)
	if fallbackErr != nil {
		return nil, fmt.Errorf("%w; fallback lookup failed: %w", err, fallbackErr)
	}
	if fallbackSessionKey == 0 || fallbackSessionKey == request.SessionKey {
		return nil, err
	}

	fallbackRequest := request
	fallbackRequest.SessionKey = fallbackSessionKey
	endpoint, err = c.buildURL("starting_grid", fallbackRequest)
	if err != nil {
		return nil, err
	}

	rows, err = c.fetchWithRetry(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row["session_key"] = request.SessionKey
		row["meeting_key"] = request.MeetingKey
		row["source_session_key"] = fallbackSessionKey
	}

	return rows, nil
}

// fetchWithRetry executes one provider endpoint with the client's retry policy: retryable
// failures are retried up to c.retries times with c.retryDelay between attempts, and a 429 waits
// for its Retry-After (bounded) without consuming a retry, up to maxRateLimitWaits times.
func (c *Client) fetchWithRetry(ctx context.Context, endpoint string) ([]map[string]any, error) {
	var lastErr error
	attempt, rateLimited := 0, 0
	for {
		rows, err := c.fetchOnce(ctx, endpoint)
		if err == nil {
			return rows, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		wait := c.retryDelay
		var upstream *upstreamError
		switch {
		case errors.As(err, &upstream) && upstream.status == http.StatusTooManyRequests && rateLimited < maxRateLimitWaits:
			rateLimited++
			if upstream.retryAfter > 0 {
				wait = upstream.retryAfter
			}
		case errors.As(err, &upstream) && !upstream.retryable():
			return nil, err
		case errors.Is(err, bootstrap.ErrResponseTooLarge):
			return nil, err
		default:
			if attempt == c.retries {
				return nil, lastErr
			}
			attempt++
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// findStartingGridSession finds the qualifying session that provides the grid for a race session.
func (c *Client) findStartingGridSession(ctx context.Context, request domain.OpenF1IngestionRequest) (int, error) {
	query := url.Values{}
	query.Set("meeting_key", strconv.Itoa(request.MeetingKey))

	sessions, err := c.fetchWithRetry(ctx, fmt.Sprintf("%s/sessions?%s", c.baseURL, query.Encode()))
	if err != nil {
		return 0, err
	}

	targetIndex := -1
	targetName := ""
	for index, session := range sessions {
		if intValue(session["session_key"]) == request.SessionKey {
			targetIndex = index
			targetName = strings.ToLower(strings.TrimSpace(asString(session["session_name"])))
			break
		}
	}
	if targetIndex == -1 {
		return 0, fmt.Errorf("session %d not found in meeting %d", request.SessionKey, request.MeetingKey)
	}

	if strings.Contains(targetName, "sprint") {
		for index := targetIndex - 1; index >= 0; index-- {
			name := strings.ToLower(strings.TrimSpace(asString(sessions[index]["session_name"])))
			if strings.Contains(name, "sprint") && strings.Contains(name, "qualifying") {
				return intValue(sessions[index]["session_key"]), nil
			}
		}
	}

	for index := targetIndex - 1; index >= 0; index-- {
		name := strings.ToLower(strings.TrimSpace(asString(sessions[index]["session_name"])))
		sessionType := strings.ToLower(strings.TrimSpace(asString(sessions[index]["session_type"])))
		if strings.Contains(name, "sprint") {
			continue
		}
		if sessionType == "qualifying" || strings.Contains(name, "qualifying") {
			return intValue(sessions[index]["session_key"]), nil
		}
	}

	return 0, fmt.Errorf("no qualifying session found before session %d", request.SessionKey)
}

// buildURL builds the upstream request URL for the requested provider resource.
func (c *Client) buildURL(resource string, request domain.OpenF1IngestionRequest) (string, error) {
	allowed := map[string]bool{
		"meetings":             true,
		"sessions":             true,
		"drivers":              true,
		"championship_drivers": true,
		"championship_teams":   true,
		"session_result":       true,
		"starting_grid":        true,
		"laps":                 true,
		"car_data":             true,
		"location":             true,
		"position":             true,
		"intervals":            true,
		"stints":               true,
		"pit":                  true,
		"weather":              true,
		"team_radio":           true,
		"overtakes":            true,
		"race_control":         true,
	}
	if !allowed[resource] {
		return "", fmt.Errorf("%w %q", domain.ErrUnsupportedResource, resource)
	}

	query := url.Values{}
	switch resource {
	case "meetings":
		query.Set("meeting_key", strconv.Itoa(request.MeetingKey))
	case "sessions":
		query.Set("meeting_key", strconv.Itoa(request.MeetingKey))
		query.Set("session_key", strconv.Itoa(request.SessionKey))
	case "drivers":
		query.Set("session_key", strconv.Itoa(request.SessionKey))
	case "championship_drivers", "session_result", "starting_grid", "championship_teams":
		query.Set("session_key", strconv.Itoa(request.SessionKey))
		if resource == "championship_drivers" && request.DriverNumber != nil {
			query.Set("driver_number", strconv.Itoa(*request.DriverNumber))
		}
	default:
		query.Set("session_key", strconv.Itoa(request.SessionKey))
		if request.DriverNumber != nil {
			query.Set("driver_number", strconv.Itoa(*request.DriverNumber))
		}
	}

	return fmt.Sprintf("%s/%s?%s", c.baseURL, resource, query.Encode()), nil
}

// fetchOnce executes one upstream HTTP request (after the pacing delay) and decodes the JSON
// response rows, refusing a body over c.maxResponseBytes.
func (c *Client) fetchOnce(ctx context.Context, endpoint string) ([]map[string]any, error) {
	if err := c.pace(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, &upstreamError{
			status:     resp.StatusCode,
			body:       strings.TrimSpace(string(body)),
			retryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
		}
	}

	var rows []map[string]any
	if err := json.NewDecoder(bootstrap.LimitResponse(resp.Body, c.maxResponseBytes)).Decode(&rows); err != nil {
		if errors.Is(err, bootstrap.ErrResponseTooLarge) {
			return nil, fmt.Errorf("%w: %w", domain.ErrResponseTooLarge, err)
		}
		return nil, err
	}

	return rows, nil
}

// pace waits until c.minInterval has passed since the previous request, so a full-session
// ingestion (dozens of calls) stays under the OpenF1 rate limit.
func (c *Client) pace(ctx context.Context) error {
	if c.minInterval <= 0 {
		return nil
	}
	c.paceMu.Lock()
	defer c.paceMu.Unlock()
	wait := time.Until(c.lastCall.Add(c.minInterval))
	if wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	c.lastCall = time.Now()
	return nil
}

// parseRetryAfter reads a Retry-After header (seconds or HTTP date), capped at maxRetryAfter;
// zero when absent or invalid.
func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var wait time.Duration
	if seconds, err := strconv.Atoi(value); err == nil {
		wait = time.Duration(seconds) * time.Second
	} else if at, err := http.ParseTime(value); err == nil {
		wait = at.Sub(now)
	}
	if wait <= 0 {
		return 0
	}
	return min(wait, maxRetryAfter)
}

// intValue converts OpenF1 numeric payload values into an int.
func intValue(value any) int {
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
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return 0
}
