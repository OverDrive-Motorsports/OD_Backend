/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## dispatcher.go - Package httpdispatcher source file for services/ingestion-service/src/adapters/dispatch/http.
	##
*/

package httpdispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	contracts "overdrive/shared/contracts/ingestion"
)

type Dispatcher struct {
	httpClient *http.Client
	endpoints  map[string]string
}

// NewDispatcher builds and returns a dispatcher with its required dependencies.
func NewDispatcher(raceDataServiceURL string, championshipServiceURL string, timeout time.Duration) *Dispatcher {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	return &Dispatcher{
		httpClient: &http.Client{Timeout: timeout},
		endpoints: map[string]string{
			"race-data-service":    strings.TrimRight(raceDataServiceURL, "/") + "/internal/ingestion/batches",
			"championship-service": strings.TrimRight(championshipServiceURL, "/") + "/internal/ingestion/batches",
		},
	}
}

// Dispatch sends an ingestion batch to the configured downstream service endpoint.
func (d *Dispatcher) Dispatch(ctx context.Context, service string, batch contracts.Batch) (contracts.DispatchAck, error) {
	endpoint := d.endpoints[service]
	if endpoint == "" {
		return contracts.DispatchAck{}, fmt.Errorf("no dispatcher endpoint configured for %s", service)
	}

	body, err := json.Marshal(batch)
	if err != nil {
		return contracts.DispatchAck{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return contracts.DispatchAck{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return contracts.DispatchAck{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return contracts.DispatchAck{}, fmt.Errorf("dispatch rejected with status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var ack contracts.DispatchAck
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return contracts.DispatchAck{}, err
	}

	return ack, nil
}
