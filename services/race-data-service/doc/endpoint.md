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

### Live race routes (public contract, validated 2026-07-08)

All responses below are **camelCase** JSON. List endpoints return **bare JSON
arrays** (`[...]`), never a `{ "count", "data" }` envelope — see
`.story/endpoint.md` for the exact field-by-field shape of each response.
`sessionId` is validated against `championship-service`; unknown sessions
return `404 { "error": "session not found" }`.

- `GET /sessions/{sessionId}/race/position` (query: `driverNumber`, `lapNumber`) — returns a single position object when `driverNumber` is given, otherwise a bare array of every driver's latest position
- `GET /sessions/{sessionId}/race/laps` (query: `driverNumber`, `lapNumber`) — returns a single `{driverNumber, laps, bestLap, averageLap}` object when `driverNumber` is given, otherwise a bare array of that object per driver. `bestLap`/`averageLap` are always computed from ALL of that driver's laps, even when `lapNumber` narrows the returned `laps` list.
- `GET /sessions/{sessionId}/race/stints` (query: `driverNumber`) — bare array
- `GET /sessions/{sessionId}/race/pitstops` (query: `driverNumber`) — bare array
- `POST /sessions/{sessionId}/race/control` — **long-poll**: blocks (up to a bounded server-side timeout) until a new race control event batch is ingested for the session, returns it as a bare array, then closes. The client must re-POST to receive the next event. Implemented via an in-memory, single-process pub/sub (`src/core/usecases/race_control_broadcaster.go`) — **this only works correctly with a single race-data-service instance**; horizontal scaling requires swapping in a shared/external broker.
- `GET /sessions/{sessionId}/race/weather` — bare array, no filters
- `GET /sessions/{sessionId}/race/radio` (query: `driverNumber`) — bare array

Known gap: `RaceControlEvent.penality` (driverNumber/timePenalty) has no upstream
data source and is always `null`. `safetyCar` (`"SC"`/`"VSC"`/`null`) is derived
with a best-effort heuristic from the OpenF1 `category`/`scope` fields, since
OpenF1 does not expose a dedicated safety-car flag.

### Live telemetry routes (public contract, validated 2026-07-08)

`battery` is intentionally **absent** from `/telemetry/engine` — no data source
is currently ingested for it.

- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/speed` (query: `lapNumber`) — single object; `topSpeed`/`averageSpeed` are computed from the scoped samples (whole session, or a single lap when `lapNumber` is given)
- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/engine` (query: `lapNumber`) — single object; `drsActive` is a best-effort heuristic (`drsState >= 10`) since OpenF1 encodes DRS as a numeric state code, not a boolean
- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/location` (query: `lapNumber`) — bare array
- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/intervals` — single object (latest interval sample)

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
