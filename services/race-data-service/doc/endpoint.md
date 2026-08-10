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

### Live race routes (public contract, validated)

All responses below are **camelCase** JSON. List endpoints return **bare JSON
arrays** (`[...]`), never a `{ "count", "data" }` envelope. `sessionId` is
validated against `championship-service`; unknown sessions return `404
{ "error": { "code": "SESSION_NOT_FOUND", "status": 404, "message": "session
not found" } }` — the standard `shared/apierror` envelope used across the
gateway, championship-service, race-data-service, and ingestion-service (see
`shared/apierror/http.go`).

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

### Replay route: `GET /sessions/{sessionId}/race/replay`

A plain JSON **bulk dump** of every stored sample for a session, across eight
datasets, in one response. It is additive to every route above — the
snapshot endpoints are unchanged and remain the right choice for a widget's
initial load or a small, targeted query. This route exists so a page (or an
offline analysis job) that wants the whole session's history at once can
fetch it in a single request instead of calling every snapshot endpoint in
a loop.

It is a **replay of already-ingested historical data**, not a live feed —
there is no in-progress race in this dev setup. All rows for a
finished/stored session are returned as soon as they're assembled; there is
no pacing, streaming, or partial-response behavior.

Query params:

- `driverNumber` (optional int) — scope driver-scoped datasets (`telemetry`,
  `position`, `radio`, `pitStop`, `stint`, `lap`) to one driver: each of
  those datasets' objects will have at most one key (that driver, if they
  have data). `raceControl` and `weather` are track-wide and are always
  included in full regardless of this filter (matching `POST /race/control`
  and `GET /race/weather`, neither of which takes a `driverNumber` either).

`sessionId` is validated against championship-service the same way as every
other `/race/*`/`/telemetry/*` route; unknown sessions return `404`.

Response body is a single JSON object with one key per dataset. Driver-scoped
datasets (`telemetry`, `position`, `radio`, `pitStop`, `stint`, `lap`) are
objects keyed by **driver number as a JSON string** (e.g. `"63"`), each
mapping to a chronologically-sorted (oldest first) array of that driver's
samples — `driverNumber` is dropped from each nested sample since it's
already implied by the outer key. Track-wide datasets (`raceControl`,
`weather`) stay flat arrays, same shape as their snapshot endpoints
(`GET /race/control`, `GET /race/weather`):

- `telemetry` — same fields as a `GET /telemetry/speed`/`GET /telemetry/engine` sample, minus `driverNumber`
- `position` — same fields as a `GET /race/position` row, minus `driverNumber`; gap fields (`gapToLeader`/`gapAhead`/`gapBehind`) are intentionally left empty, same as the position snapshot's underlying samples — recomputing them per historical row isn't worth an extra lookup per sample
- `raceControl` — same shape as `GET /race/control`, flat array, track-wide
- `weather` — same shape as `GET /race/weather`, flat array, track-wide
- `radio` — same fields as a `GET /race/radio` row, minus `driverNumber`; rows with no recorded `dateUtc` (nullable) are skipped, not guessed at
- `pitStop` — same fields as a `GET /race/pitstops` row, minus `driverNumber`; rows with no recorded `dateUtc` (nullable) are skipped
- `stint` — same fields as a `GET /race/stints` row, minus `driverNumber`. **`RaceStint` has no timestamp column at all** in the schema (only `stintNumber`/`lapStart`/`lapEnd`/`compound`/`tyreAgeLapsStart`), so each stint's position within its driver's array is an **approximation**: it is sorted by the recorded start time (`dateStartUtc`) of its `lapStart` lap. A stint whose `lapStart` lap has no recorded `dateStartUtc` is skipped entirely rather than placed with a guessed time.
- `lap` — same fields as a `GET /race/laps` entry, minus `driverNumber`; one entry per completed lap, ordered by lap **completion** (`dateStartUtc + lapDurationSec`), not lap start, since duration/sectors are only known once the lap finishes. Laps missing either `dateStartUtc` or `lapDurationSec` (both nullable) are skipped.

Example:

```bash
curl http://localhost:3004/sessions/<SESSION_ID>/race/replay?driverNumber=63
```

```json
{
  "telemetry": {
    "63": [
      { "speed": 312, "rpm": 11800, "gear": 7, "throttlePercent": 100, "brakePercent": 0, "drsActive": true, "lapNumber": 12, "x": 1024.5, "y": -302.1, "z": 12.3, "timestamp": "2026-03-08T15:12:04Z" }
    ]
  },
  "position": {
    "63": [
      { "position": 1, "gapAhead": null, "gapBehind": null, "lapsCompleted": 12, "timestamp": "2026-03-08T15:12:04Z" }
    ]
  },
  "radio": { "63": [] },
  "pitStop": { "63": [] },
  "stint": {
    "63": [
      { "stintNumber": 1, "compound": "MEDIUM", "lapStart": 1, "lapEnd": 20, "tyreAgeAtStart": 0 }
    ]
  },
  "lap": {
    "63": [
      { "lapNumber": 12, "lapDuration": 91.204, "sector1": 28.1, "sector2": 33.4, "sector3": 29.7, "isPitOutLap": false }
    ]
  },
  "raceControl": [
    { "category": "Flag", "flag": "GREEN", "message": "TRACK CLEAR", "lapNumber": 1, "timestamp": "2026-03-08T15:00:00Z", "penality": null, "safetyCar": null }
  ],
  "weather": [
    { "timestamp": "2026-03-08T15:00:00Z", "airTemperature": 24.1, "trackTemperature": 31.5, "humidity": 48, "windSpeed": 1.2, "rainfall": false }
  ]
}
```

Implementation: `src/adapters/http/race_replay_stream.controller.go`
(request parsing + JSON response, no streaming machinery),
`src/core/usecases/race_replay_stream.usecase.go` (groups each dataset's
already-sorted rows by driver number), `src/adapters/repository/prisma/
race_stream.repository.go` (oldest-first Prisma reads, including the `stint`
lap-start anchoring and `lap` completion-time derivation). The gateway's
`race_parameter_middleware.go` allow-list accepts `replay` as one of the
6-segment `/race/{action}` routes — see `gateway/ROUTES.md`.

### Live telemetry routes (public contract, validated)

`battery` is intentionally **absent** from `/telemetry/engine` — no data source
is currently ingested for it.

- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/speed` (query: `lapNumber`) — bare array of every sample, oldest first (whole session, or a single lap when `lapNumber` is given), same shape family as `/telemetry/location` — not an aggregated `{currentSpeed, topSpeed, averageSpeed}` object.
- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/engine` (query: `lapNumber`) — bare array of every sample, oldest first, not just the latest one. `drsActive` is a best-effort heuristic (`drsState >= 10`) since OpenF1 encodes DRS as a numeric state code, not a boolean
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
