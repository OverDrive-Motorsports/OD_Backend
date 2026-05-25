# race-data-service endpoints

## `GET /health`

Returns the runtime status of the service.

## `POST /internal/ingestion/batches`

Internal endpoint used by `ingestion-service` to persist normalized race-oriented datasets.

Accepted dataset names:

- `lap_timing`
- `telemetry`
- `location`
- `position`
- `intervals`
- `stints`
- `pit_stops`
- `weather`
- `team_radio`
- `overtakes`
- `race_control`

## Public facade endpoints

`race-data-service` now exposes the session-scoped read facade used by the previous backend.  
Catalog routes are proxied to `championship-service`; race datasets are read locally and enriched with championship metadata.

### Catalog proxy

- `GET /championships`
- `GET /championships/{code}/events`
- `GET /events/{eventId}`
- `GET /events/{eventId}/sessions`

### Session routes

- `GET /sessions/{sessionId}`
- `GET /sessions/{sessionId}/metadata`
- `GET /sessions/{sessionId}/drivers`
- `GET /sessions/{sessionId}/teams`
- `GET /sessions/{sessionId}/datasets/{dataset}`
- `GET /sessions/{sessionId}/standings/race`
- `GET /sessions/{sessionId}/broadcast`
- `GET /sessions/{sessionId}/weather`
- `GET /sessions/{sessionId}/facts`

### Driver routes

- `GET /sessions/{sessionId}/drivers/{driverNumber}/profile`
- `GET /sessions/{sessionId}/drivers/{driverNumber}/broadcast`
- `GET /sessions/{sessionId}/drivers/{driverNumber}/{dataset}`
- `GET /sessions/{sessionId}/drivers/{driverNumber}/laps/{lapNumber}/location`

Supported driver datasets:

- `laps`
- `telemetry`
- `location`
- `position`
- `intervals`
- `stints`
- `pit`
- `radio`
- `result`

Supported session datasets:

- local:
  - `laps`
  - `car_data`
  - `telemetry`
  - `location`
  - `position`
  - `intervals`
  - `stints`
  - `pit`
  - `weather`
  - `team_radio`
  - `overtakes`
  - `race_control`
- proxied from `championship-service`:
  - `session_result`
  - `starting_grid`
  - `championship_drivers`
  - `championship_teams`

### Reserved tables

The Prisma schema also contains archive and highlight-oriented tables:

- `RaceArchive`
- `RaceDatasetChunk`
- `RaceHighlight`

These are reserved for future precomputed archive/highlight workflows. They are not populated by the current OpenF1 ingestion endpoint.

### Example

```bash
curl http://localhost:3004/championships
curl http://localhost:3004/sessions/<SESSION_ID>/datasets/laps
curl http://localhost:3004/sessions/<SESSION_ID>/drivers/63/telemetry
curl http://localhost:3004/sessions/<SESSION_ID>/drivers/63/laps/27/location
```
