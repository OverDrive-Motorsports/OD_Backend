/**
##
## OverDrive 2026
## All Technical rights reserved
##
## client.go - Package championshipclient source file for services/race-data-service/src/adapters/downstream/championship.
##
*/

package championshipclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"overdrive/services/race-data-service/src/core/ports"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New builds and returns a  with its required dependencies.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ListChampionships returns a collection of championships for the requested context.
// championship-service now returns a bare JSON array; forward it unchanged.
func (c *Client) ListChampionships(ctx context.Context) (any, error) {
	var payload []map[string]any
	ok, err := c.getJSON(ctx, "/championships", &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// ListChampionshipEvents returns a collection of championship events for the requested context.
func (c *Client) ListChampionshipEvents(ctx context.Context, code string) (any, error) {
	var payload []map[string]any
	ok, err := c.getJSON(ctx, "/championships/"+code+"/events", &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// GetEventPayload returns the requested event payload payload for the supplied identifiers.
func (c *Client) GetEventPayload(ctx context.Context, eventID string) (any, error) {
	var payload map[string]any
	ok, err := c.getJSON(ctx, "/events/"+eventID, &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// ListEventSessions returns a collection of event sessions for the requested context.
func (c *Client) ListEventSessions(ctx context.Context, eventID string) (any, error) {
	var payload []map[string]any
	ok, err := c.getJSON(ctx, "/events/"+eventID+"/sessions", &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// GetSession returns the requested session payload for the supplied identifiers.
func (c *Client) GetSession(ctx context.Context, sessionID string) (*ports.ChampionshipSessionRef, error) {
	var payload ports.ChampionshipSessionRef
	ok, err := c.getJSON(ctx, "/sessions/"+sessionID, &payload)
	if err != nil || !ok {
		return nil, err
	}
	return &payload, nil
}

// GetEvent returns the requested event payload for the supplied identifiers.
func (c *Client) GetEvent(ctx context.Context, eventID string) (*ports.ChampionshipEventRef, error) {
	var payload ports.ChampionshipEventRef
	ok, err := c.getJSON(ctx, "/events/"+eventID, &payload)
	if err != nil || !ok {
		return nil, err
	}
	return &payload, nil
}

// ListSessionDrivers returns a collection of session drivers for the requested context.
// championship-service now returns a bare JSON array (camelCase fields).
func (c *Client) ListSessionDrivers(ctx context.Context, sessionID string) ([]ports.ChampionshipDriverRef, error) {
	var payload []ports.ChampionshipDriverRef
	ok, err := c.getJSON(ctx, "/sessions/"+sessionID+"/drivers", &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// ListSessionTeams returns a collection of session teams for the requested context.
func (c *Client) ListSessionTeams(ctx context.Context, sessionID string) ([]map[string]any, error) {
	var payload []map[string]any
	ok, err := c.getJSON(ctx, "/sessions/"+sessionID+"/teams", &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// GetSessionDataset returns the requested session dataset payload for the supplied identifiers.
func (c *Client) GetSessionDataset(ctx context.Context, sessionID string, dataset string) (map[string]any, error) {
	var payload map[string]any
	ok, err := c.getJSON(ctx, "/sessions/"+sessionID+"/datasets/"+dataset, &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// GetSessionRaceStandings returns the requested session race standings payload for the supplied identifiers.
func (c *Client) GetSessionRaceStandings(ctx context.Context, sessionID string) (any, error) {
	var payload map[string]any
	ok, err := c.getJSON(ctx, "/sessions/"+sessionID+"/standings/race", &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// GetSessionBroadcast returns the requested session broadcast payload for the supplied identifiers.
func (c *Client) GetSessionBroadcast(ctx context.Context, sessionID string) (map[string]any, error) {
	var payload map[string]any
	ok, err := c.getJSON(ctx, "/sessions/"+sessionID+"/broadcast", &payload)
	if err != nil || !ok {
		return nil, err
	}
	return payload, nil
}

// getJSON implements the get json workflow for this package.
func (c *Client) getJSON(ctx context.Context, path string, target any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return false, fmt.Errorf("championship-service GET %s returned %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return false, err
	}
	return true, nil
}
