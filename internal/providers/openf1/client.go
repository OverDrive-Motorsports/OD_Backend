/**
##
## OverDrive 2026
## All Technical rights reserved
##
## client.go - OpenF1 HTTP client with retries, throttling, and response decoding.
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
	"path"
	"strings"
	"sync"
	"time"
)

type Client struct {
	baseURL       string
	httpClient    *http.Client
	requestGap    time.Duration
	maxRetries    int
	retryDelay    time.Duration
	mu            sync.Mutex
	lastRequestAt time.Time
}

// HTTPError captures provider HTTP failures so callers can apply business fallbacks.
type HTTPError struct {
	Endpoint   string
	StatusCode int
	Body       string
}

// Error formats an OpenF1 HTTP error with endpoint, status, and trimmed response body.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("openf1 %s failed status=%d body=%s", e.Endpoint, e.StatusCode, e.Body)
}

// NewClient creates an OpenF1 client with retry and throttling settings.
func NewClient(baseURL string, timeout, requestGap time.Duration, maxRetries int, retryDelay time.Duration) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		requestGap: requestGap,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// Get fetches an OpenF1 endpoint and decodes its JSON array response.
func (c *Client) Get(ctx context.Context, endpoint string, params map[string]string) ([]map[string]any, error) {
	targetURL, err := c.buildURL(endpoint, params)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := c.waitForGap(ctx); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create request for %s: %w", endpoint, err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request %s: %w", endpoint, err)
			if attempt < c.maxRetries {
				if sleepErr := sleepWithContext(ctx, c.retryDelay); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			break
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read body for %s: %w", endpoint, readErr)
			if attempt < c.maxRetries {
				if sleepErr := sleepWithContext(ctx, c.retryDelay); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			break
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			lastErr = fmt.Errorf("transient status on %s: %d", endpoint, resp.StatusCode)
			if attempt < c.maxRetries {
				if sleepErr := sleepWithContext(ctx, c.retryDelay); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			break
		}

		if resp.StatusCode >= http.StatusBadRequest {
			return nil, &HTTPError{
				Endpoint:   endpoint,
				StatusCode: resp.StatusCode,
				Body:       trimBody(body, 320),
			}
		}

		rows := make([]map[string]any, 0)
		trimmed := strings.TrimSpace(string(body))
		if trimmed == "" || trimmed == "null" {
			return rows, nil
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, fmt.Errorf("decode %s response: %w", endpoint, err)
		}
		return rows, nil
	}

	if lastErr == nil {
		lastErr = errors.New("unknown openf1 client error")
	}
	return nil, fmt.Errorf("openf1 %s failed after retries: %w", endpoint, lastErr)
}

// buildURL constructs a full endpoint URL with encoded query parameters.
func (c *Client) buildURL(endpoint string, params map[string]string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	base.Path = path.Join(base.Path, endpoint)
	query := base.Query()
	for k, v := range params {
		query.Set(k, v)
	}
	base.RawQuery = query.Encode()
	return base.String(), nil
}

// waitForGap enforces a minimum delay between outbound provider requests.
func (c *Client) waitForGap(ctx context.Context) error {
	if c.requestGap <= 0 {
		return nil
	}

	c.mu.Lock()
	last := c.lastRequestAt
	c.mu.Unlock()

	if !last.IsZero() {
		elapsed := time.Since(last)
		if elapsed < c.requestGap {
			wait := c.requestGap - elapsed
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	c.mu.Lock()
	c.lastRequestAt = time.Now()
	c.mu.Unlock()
	return nil
}

// sleepWithContext sleeps for a duration while still honoring context cancellation.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// trimBody truncates large response bodies for safer error messages.
func trimBody(body []byte, max int) string {
	raw := strings.TrimSpace(string(body))
	if len(raw) <= max {
		return raw
	}
	return raw[:max] + "..."
}
