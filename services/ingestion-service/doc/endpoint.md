# ingestion-service endpoints

Through the gateway the routes are under `/v1/ingestion` and are restricted to administrators:
`Authorization: Bearer <access token>` whose `role` claim is `admin`, or
`Authorization: Bearer <GATEWAY_AUTH_TOKEN>` for scripted calls. A valid `user` token answers
`403 FORBIDDEN`; no, expired or invalid token answers `401` (see `gateway/ROUTES.md`). Promoting
an account to `admin` is described in `services/auth-service/doc/endpoint.md`. The service
itself checks no token: it relies on the gateway.

## `GET /health`

Returns the runtime status of the service.

## `POST /providers/openf1/ingestions`

Triggers a manual OpenF1 ingestion batch.

### Request body

```json
{
  "meeting_key": 1255,
  "session_key": 9998,
  "driver_number": 63,
  "resources": ["drivers", "laps", "car_data", "position"],
  "dispatch": true
}
```

### Fields

| Field | Type | Mandatory | Description |
| --- | --- | --- | --- |
| `meeting_key` | `int` | Yes | OpenF1 meeting identifier |
| `session_key` | `int` | Yes | OpenF1 session identifier |
| `driver_number` | `int` | No | Restricts driver-scoped datasets to one driver |
| `resources` | `string[]` | No | Explicit list of OpenF1 resources to fetch. If omitted, all supported resources are fetched |
| `dispatch` | `bool` | No | If `false`, fetch and mapping still run but no downstream dispatch is performed |

### Supported OpenF1 resources

- `meetings`
- `sessions`
- `drivers`
- `championship_drivers`
- `championship_teams`
- `session_result`
- `starting_grid`
- `laps`
- `car_data`
- `location`
- `position`
- `intervals`
- `stints`
- `pit`
- `weather`
- `team_radio`
- `overtakes`
- `race_control`

### Persistence mapping

| OpenF1 resource | Target service | Main Prisma table(s) |
| --- | --- | --- |
| `meetings` | `championship-service` | `Event` |
| `sessions` | `championship-service` | `Session` |
| `drivers` | `championship-service` | `Driver`, `Team` |
| `session_result` | `championship-service` | `SessionResultRow` |
| `starting_grid` | `championship-service` | `StartingGridRow` |
| `championship_drivers` | `championship-service` | `DriverChampionshipStanding` |
| `championship_teams` | `championship-service` | `TeamChampionshipStanding` |
| `laps` | `race-data-service` | `RaceLap`, `SessionDriverBroadcast` |
| `car_data` | `race-data-service` | `RaceTelemetrySample`, `SessionDriverBroadcast` |
| `location` | `race-data-service` | `RaceLocationSample`, `SessionDriverBroadcast` |
| `position` | `race-data-service` | `RacePositionSample`, `SessionDriverBroadcast` |
| `intervals` | `race-data-service` | `RaceIntervalSample`, `SessionDriverBroadcast` |
| `stints` | `race-data-service` | `RaceStint`, `SessionDriverBroadcast` |
| `pit` | `race-data-service` | `RacePitStop`, `SessionDriverBroadcast` |
| `weather` | `race-data-service` | `RaceWeatherSample` |
| `team_radio` | `race-data-service` | `TeamRadioMessage`, `SessionDriverBroadcast` |
| `overtakes` | `race-data-service` | `OvertakeEvent` |
| `race_control` | `race-data-service` | `RaceControlEvent` |

### Recommended ingestion order

Ingest catalog data before standings or race-data datasets:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["meetings","sessions","drivers","session_result"],"dispatch":true}'
```

Then add championship standings and grid data:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["championship_drivers","championship_teams","starting_grid"],"dispatch":true}'
```

Then add race-data datasets. Large resources should be sent in smaller batches:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["position","intervals","stints","pit","weather","overtakes","race_control"],"dispatch":true}'
```

`car_data` and `location` are the heavy resources (a whole session is several hundred
megabytes). Without `driver_number`, the service fetches the session's driver list, then fetches,
maps and dispatches each heavy resource **one driver at a time**: the full-session request below
works, it just takes several minutes. With `driver_number`, it makes a single call for that
driver:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"driver_number":63,"resources":["car_data","location","team_radio"],"dispatch":true}'
```

A full-session ingestion, every supported resource at once:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"dispatch":true}'
```

### OpenF1 pacing, retries and limits

- Requests to OpenF1 are spaced by `OPENF1_MIN_REQUEST_INTERVAL` (default `350ms`).
- A `429` is retried after its `Retry-After` (capped at 60 s), up to 5 times per request, without
  using the normal retries. Server errors (`5xx`), `408` and network errors are retried
  `OPENF1_RETRY_COUNT` times, `OPENF1_RETRY_DELAY` apart. A `404` ("No results found") means the
  resource is empty; any other `4xx` fails at once.
- An empty resource (or an empty driver of a heavy resource, or a resource whose rows were all
  rejected by the mapping) is reported `ok` with `row_count: 0`, `dispatched: false`; an
  empty `sessions` resource means OpenF1 does not know the session: the run stops before any
  dispatch and answers `404`: several datasets replace the session's rows, so an empty batch would
  wipe what an earlier ingestion stored.
- One OpenF1 response is limited to `OPENF1_MAX_RESPONSE_BYTES` (default 128 MiB); a larger one
  fails its resource with `OpenF1 response too large` instead of being loaded.
- The request may run up to 30 minutes (the server write deadline of this route); the gateway
  route `/v1/ingestion` allows the same.

### Starting grid fallback

OpenF1 does not always expose `starting_grid` directly on Race sessions. When `starting_grid` returns no rows for a Race session, the provider adapter looks for the relevant qualifying session from the same meeting and persists that grid against the requested Race session.

For sprint sessions, the adapter first looks for the preceding Sprint Qualifying session. For normal Race sessions, it looks for the preceding non-sprint Qualifying session.

### Returned message

Status: `202 Accepted` when at least one resource was ingested. A failing resource does not
stop the others: each one reports its own `status`.

| Field | Meaning |
| --- | --- |
| `status` | `completed` (every resource `ok`) or `partial` |
| `resources[].status` | `ok`, `partial` (a heavy resource where some drivers failed) or `failed` |
| `resources[].error` | short reason when not `ok`: `OpenF1 request failed`, `OpenF1 request timed out`, `OpenF1 response too large`, `mapping failed`, `dispatch to <service> failed`, `driver list unavailable: ...`, `every driver failed`, `N of M drivers failed` (details in the service log) |
| `resources[].failed_drivers` | drivers that failed for a heavy resource |

```json
{
  "status": "completed",
  "provider": "openf1",
  "batch_id": "openf1-1770000000000",
  "meeting_key": 1255,
  "session_key": 9998,
  "resources": [
    {
      "resource": "drivers",
      "dataset_name": "driver_catalog",
      "target_service": "championship-service",
      "row_count": 20,
      "dispatched": true,
      "status": "ok"
    }
  ],
  "dispatches": [
    {
      "service": "championship-service",
      "dataset_count": 1,
      "row_count": 20
    }
  ],
  "dispatch_enabled": true,
  "triggered_at_utc": "YYYY-MM-DDThh:mm:ssZ",
  "completed_at_utc": "YYYY-MM-DDThh:mm:ssZ"
}
```

### Errors

| Status | Code | When |
| --- | --- | --- |
| `400` | `VALIDATION_ERROR` | malformed body, `meeting_key`/`session_key` not positive, unknown resource name (checked before any OpenF1 call) |
| `404` | `OPENF1_DATA_NOT_FOUND` | OpenF1 does not know this session (`sessions` empty) or every resource came back empty; nothing is dispatched |
| `413` | `PAYLOAD_TOO_LARGE` | body over 1 MiB |
| `502` | `UPSTREAM_UNAVAILABLE` | every resource failed (nothing ingested) |
| `504` | `UPSTREAM_TIMEOUT` | the request context expired during the run |
