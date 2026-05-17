/*
*

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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	retries    int
	retryDelay time.Duration
}

// NewClient builds and returns a client with its required dependencies.
func NewClient(baseURL string, timeout time.Duration, retries int, retryDelay time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retries:    retries,
		retryDelay: retryDelay,
	}
}

// Fetch retrieves provider data for the requested resource and ingestion context.
func (c *Client) Fetch(ctx context.Context, resource string, request domain.OpenF1IngestionRequest) ([]map[string]any, error) {
	if resource == "starting_grid" {
		return c.fetchStartingGrid(ctx, request)
	}

	endpoint, err := c.buildURL(resource, request)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		rows, err := c.fetchOnce(ctx, endpoint)
		if err == nil {
			return rows, nil
		}

		lastErr = err
		if attempt == c.retries {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.retryDelay):
		}
	}

	return nil, lastErr
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
	if !strings.Contains(err.Error(), "status 404") {
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

// fetchWithRetry executes one provider endpoint with the client's retry policy.
func (c *Client) fetchWithRetry(ctx context.Context, endpoint string) ([]map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		rows, err := c.fetchOnce(ctx, endpoint)
		if err == nil {
			return rows, nil
		}

		lastErr = err
		if attempt == c.retries {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.retryDelay):
		}
	}

	return nil, lastErr
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
		return "", fmt.Errorf("unsupported OpenF1 resource %q", resource)
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

// fetchOnce executes one upstream HTTP request and decodes the JSON response rows.
func (c *Client) fetchOnce(ctx context.Context, endpoint string) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("upstream request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	return rows, nil
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
