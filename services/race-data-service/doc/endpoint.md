# race-data-service endpoints

Through the gateway the public routes are under `/v1/race-data` and require the user's access
token (`Authorization: Bearer <access token>`, any role); the gateway token is refused. Without
a token: `401 UNAUTHORIZED`, expired: `401 TOKEN_EXPIRED`, otherwise invalid:
`401 INVALID_TOKEN` (see `gateway/ROUTES.md`). The service itself checks no user token: it
relies on the gateway.

## `GET /health`

Returns the runtime status of the service.

## `POST /internal/ingestion/batches`

Internal endpoint used by `ingestion-service` to persist normalized race-oriented datasets.

Requires the header `X-Internal-Token: <INTERNAL_INGESTION_TOKEN>` (the shared secret of the
root `.env`, compared in constant time). Missing or wrong token: `401`
`{"error":{"code":"UNAUTHORIZED","status":401,"message":"invalid internal token"}}`, returned
before the body is read. The service refuses to start without a token of at least 32 bytes.

The body is limited to 256 MiB: a larger batch answers `413 PAYLOAD_TOO_LARGE`. The route may
read its body for up to 5 minutes and answer within 15 minutes (storage of a large batch), the
server defaults being 30 s and 60 s. A stored batch drops the cached replays of its session
(see the replay route); if that fails the batch answers `500 INTERNAL_ERROR` and must be resent.

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

### Replay catalog

`GET /championships/{code}/replays`, the championship's **playable** replays,
as a bare JSON array, newest first. Built for the VR replay page: each
`sessionId` is then opened with `GET /sessions/{sessionId}/race/replay`.

```json
[
  {
    "sessionId": "openf1:session:9472",
    "name": "Race",
    "type": "race",
    "startTime": "YYYY-MM-DDThh:mm:ssZ",
    "seasonYear": 2024,
    "eventName": "Bahrain Grand Prix",
    "circuit": "Bahrain International Circuit"
  }
]
```

- A session is listed only when this service holds at least one telemetry
  sample **and** one location sample for it, what the replay needs to animate
  cars (without location every frame sits at `x = y = z = 0`). Sessions whose
  data was never ingested, or only partially (e.g. laps without `car_data` /
  `location`), are left out. Every session type qualifies (practice,
  qualifying, sprint, race); `name` / `type` tell them apart.
- `[]` when the championship has no playable replay.
- `404` `CHAMPIONSHIP_NOT_FOUND` for an unknown championship code; `502`
  `UPSTREAM_UNAVAILABLE` when championship-service is unreachable.
- Sessions and their event context come from championship-service
  (`GET /championships/{code}/sessions`, one call); the replay check is a
  single indexed `EXISTS` query on the local sample tables, so the cost does not
  depend on how many samples a session holds.

### Pagination of the sample lists

The routes returning raw sample lists take two optional query parameters:

- `limit` - rows per page, `1` to `50000`; default `50000`.
- `offset` - rows to skip, `0` or more; default `0`.

When more rows exist, the response carries `X-Next-Offset: <offset of the next page>`; its
absence means the last page. The response shape does not change. An invalid `limit` or `offset`
is `400 VALIDATION_ERROR`. In the `{count, data}` envelopes of the dataset routes, `count` is
the number of rows in this page. Rows are in a stable order (time then id for samples, lap number for
laps, id otherwise), so consecutive pages never overlap.

Paged routes: `GET /sessions/{sessionId}/datasets/{dataset}` (local datasets; the datasets
proxied from championship-service are not paged), `GET /sessions/{sessionId}/drivers/{driverNumber}/{dataset}`
(except `result`), and `/telemetry/speed`, `/telemetry/engine`, `/telemetry/location`. One
driver's car data or location for a whole race fits in one page; a session-wide list of every
driver does not.

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
not found" } }` - the standard `shared/apierror` envelope used across the
gateway, championship-service, race-data-service, and ingestion-service (see
`shared/apierror/http.go`).

- `GET /sessions/{sessionId}/race/position` (query: `driverNumber`, `lapNumber`) - returns a single position object when `driverNumber` is given, otherwise a bare array of every driver's latest position
- `GET /sessions/{sessionId}/race/laps` (query: `driverNumber`, `lapNumber`) - returns a single `{driverNumber, laps, bestLap, averageLap}` object when `driverNumber` is given, otherwise a bare array of that object per driver. `bestLap`/`averageLap` are always computed from ALL of that driver's laps, even when `lapNumber` narrows the returned `laps` list.
- `GET /sessions/{sessionId}/race/stints` (query: `driverNumber`) - bare array
- `GET /sessions/{sessionId}/race/pitstops` (query: `driverNumber`) - bare array
- `POST /sessions/{sessionId}/race/control` - **long-poll**: blocks (up to 30 s) until a new race control event batch is ingested for the session, returns it as a bare array (`[]` on timeout), then closes. The client must re-POST to receive the next event. It works across several race-data-service instances: an ingested batch is stored in the `RaceControlNotification` table, which every instance polls every second, so a waiter on any instance is woken within about a second (`src/core/usecases/shared_race_control_broadcaster.go`). Each instance holds at most `RACE_CONTROL_MAX_WAITERS` waiters (default 2000) and `RACE_CONTROL_MAX_WAITERS_PER_SESSION` per session (default 500); beyond that the route answers `503 SERVICE_UNAVAILABLE` with `Retry-After: 5`. During a graceful shutdown the waiters are released at once with `[]`, and new ones get `503` with `Retry-After: 1`.
- `GET /sessions/{sessionId}/race/weather` - bare array, no filters
- `GET /sessions/{sessionId}/race/radio` (query: `driverNumber`) - bare array

Known gap: `RaceControlEvent.penality` (driverNumber/timePenalty) has no upstream
data source and is always `null`. `safetyCar` (`"SC"`/`"VSC"`/`null`) is derived
with a best-effort heuristic from the OpenF1 `category`/`scope` fields, since
OpenF1 does not expose a dedicated safety-car flag.

### Replay route: `GET /sessions/{sessionId}/race/replay`

A plain JSON **bulk dump** of every stored sample for a session, across eight
datasets, in one response. It is additive to every route above - the
snapshot endpoints are unchanged and remain the right choice for a widget's
initial load or a small, targeted query. This route exists so a page (or an
offline analysis job) that wants the whole session's history at once can
fetch it in a single request instead of calling every snapshot endpoint in
a loop.

It is a **replay of already-ingested historical data**, not a live feed -
there is no in-progress race in this dev setup. There is no pacing.

The object is **streamed**: it is written dataset by dataset and driver by
driver as the samples are read, so the service never holds more than one
driver's samples of one dataset in memory, whatever the session size. The
status (`200`) is sent with the first part; a failure after that aborts the
connection, so a client never receives a truncated body that parses as
complete JSON. The route may take up to 10 minutes to send the dump (the
server default is 60 s).

Limits and cache:

- At most `REPLAY_MAX_CONCURRENT` replays (default 4) are built from the
  sample tables at once per instance; beyond that the route answers
  `503 SERVICE_UNAVAILABLE` with `Retry-After: 10`. Replays served from the
  cache hold one chunk in memory each and have their own cap,
  `REPLAY_CACHE_MAX_CONCURRENT` (default 32), with the same `503` beyond it.
- A full dump (no `driverNumber`, no time window) is recorded into
  `RaceArchive` / `RaceDatasetChunk` while it is streamed, and the next
  requests (full, or one driver) are served from it without reading the
  sample tables. When two full replays are recorded at once, the first to
  finish is kept and the other copy is dropped; a complete archive is only
  deleted by an ingestion. Any ingested batch for the session (even one that
  fails part way) deletes its archives, so a replay never serves stale samples.
  An archive takes about the size of the dump in the database (tens of MB for
  a race) and stays until the next ingestion of the session. A dump served from the cache has the
  same keys and values, but the field order inside a sample object may differ.

Query params:

- `driverNumber` (optional int) - scope driver-scoped datasets (`telemetry`,
  `position`, `radio`, `pitStop`, `stint`, `lap`) to one driver: each of
  those datasets' objects will have at most one key (that driver, if they
  have data). `raceControl` and `weather` are track-wide and are always
  included in full regardless of this filter (matching `POST /race/control`
  and `GET /race/weather`, neither of which takes a `driverNumber` either).
- `fromUtc`, `toUtc` (optional RFC 3339 timestamps, inclusive, for example
  `YYYY-MM-DDThh:mm:ssZ`) - keep only the samples inside the window, in every
  dataset (stints by their anchor time, laps by their completion time). An
  invalid value, or `fromUtc` after `toUtc`, is `400 VALIDATION_ERROR`. A
  windowed replay is always read from the sample tables, never cached.

`sessionId` is validated against championship-service the same way as every
other `/race/*`/`/telemetry/*` route; unknown sessions return `404`.

Response body is a single JSON object with one key per dataset. Driver-scoped
datasets (`telemetry`, `position`, `radio`, `pitStop`, `stint`, `lap`) are
objects keyed by **driver number as a JSON string** (e.g. `"63"`), each
mapping to a chronologically-sorted (oldest first) array of that driver's
samples - `driverNumber` is dropped from each nested sample since it's
already implied by the outer key. Track-wide datasets (`raceControl`,
`weather`) stay flat arrays, same shape as their snapshot endpoints
(`GET /race/control`, `GET /race/weather`):

- `telemetry` - same fields as a `GET /telemetry/speed`/`GET /telemetry/engine` sample, minus `driverNumber`
- `position` - same fields as a `GET /race/position` row, minus `driverNumber`; gap fields (`gapToLeader`/`gapAhead`/`gapBehind`) are intentionally left empty, same as the position snapshot's underlying samples - recomputing them per historical row isn't worth an extra lookup per sample
- `raceControl` - same shape as `GET /race/control`, flat array, track-wide
- `weather` - same shape as `GET /race/weather`, flat array, track-wide
- `radio` - same fields as a `GET /race/radio` row, minus `driverNumber`; rows with no recorded `dateUtc` (nullable) are skipped, not guessed at
- `pitStop` - same fields as a `GET /race/pitstops` row, minus `driverNumber`; rows with no recorded `dateUtc` (nullable) are skipped
- `stint` - same fields as a `GET /race/stints` row, minus `driverNumber`. **`RaceStint` has no timestamp column at all** in the schema (only `stintNumber`/`lapStart`/`lapEnd`/`compound`/`tyreAgeLapsStart`), so each stint's position within its driver's array is an **approximation**: it is sorted by the recorded start time (`dateStartUtc`) of its `lapStart` lap. A stint whose `lapStart` lap has no recorded `dateStartUtc` is skipped entirely rather than placed with a guessed time.
- `lap` - same fields as a `GET /race/laps` entry, minus `driverNumber`; one entry per completed lap, ordered by lap **completion** (`dateStartUtc + lapDurationSec`), not lap start, since duration/sectors are only known once the lap finishes. Laps missing either `dateStartUtc` or `lapDurationSec` (both nullable) are skipped.

Example:

```bash
curl http://localhost:3004/sessions/<SESSION_ID>/race/replay?driverNumber=63
```

```json
{
  "telemetry": {
    "63": [
      { "speed": 312, "rpm": 11800, "gear": 7, "throttlePercent": 100, "brakePercent": 0, "drsActive": true, "lapNumber": 12, "x": 1024.5, "y": -302.1, "z": 12.3, "timestamp": "YYYY-MM-DDThh:mm:ssZ" }
    ]
  },
  "position": {
    "63": [
      { "position": 1, "gapAhead": null, "gapBehind": null, "lapsCompleted": 12, "timestamp": "YYYY-MM-DDThh:mm:ssZ" }
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
    { "category": "Flag", "flag": "GREEN", "message": "TRACK CLEAR", "lapNumber": 1, "timestamp": "YYYY-MM-DDThh:mm:ssZ", "penality": null, "safetyCar": null }
  ],
  "weather": [
    { "timestamp": "YYYY-MM-DDThh:mm:ssZ", "airTemperature": 24.1, "trackTemperature": 31.5, "humidity": 48, "windSpeed": 1.2, "rainfall": false }
  ]
}
```

Implementation: `src/adapters/http/race_replay_stream.controller.go` and
`replay_json_writer.go` (query parsing, streamed JSON object),
`src/core/usecases/race_replay_stream.usecase.go` (dump parts in order,
concurrency cap, cache recording and serving), `src/adapters/repository/prisma/
race_stream.repository.go` (oldest-first per-driver Prisma reads, including the
`stint` lap-start anchoring and `lap` completion-time derivation) and
`replay_cache.repository.go` (the cache tables). The gateway's
`race_parameter_middleware.go` allow-list accepts `replay` as one of the
6-segment `/race/{action}` routes - see `gateway/ROUTES.md`.

### Live telemetry routes (public contract, validated)

`battery` is intentionally **absent** from `/telemetry/engine` - no data source
is currently ingested for it.

- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/speed` (query: `lapNumber`, `limit`, `offset`) - bare array of every sample, oldest first (whole session, or a single lap when `lapNumber` is given), same shape family as `/telemetry/location` - not an aggregated `{currentSpeed, topSpeed, averageSpeed}` object.
- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/engine` (query: `lapNumber`, `limit`, `offset`) - bare array of every sample, oldest first, not just the latest one. `drsActive` is a best-effort heuristic (`drsState >= 10`) since OpenF1 encodes DRS as a numeric state code, not a boolean
- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/location` (query: `lapNumber`, `limit`, `offset`) - bare array, oldest first
- `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/intervals` - single object (latest interval sample)

### Cache and reserved tables

- `RaceArchive` / `RaceDatasetChunk` hold the replay cache (one archive per
  session, one chunk per dataset and driver). They are written by the replay
  route, not by the ingestion. The replay catalog does **not** rely on them,
  it checks the sample tables the replay actually reads.
- `RaceControlNotification` holds the race control batches shared by the
  instances; rows are deleted after 5 minutes.
- `RaceHighlight` is reserved for a future highlight workflow and is never written.

### Example

```bash
curl http://localhost:3004/championships
curl http://localhost:3004/sessions/<SESSION_ID>/datasets/laps
curl http://localhost:3004/sessions/<SESSION_ID>/drivers/63/telemetry
curl http://localhost:3004/sessions/<SESSION_ID>/drivers/63/laps/27/location
```
