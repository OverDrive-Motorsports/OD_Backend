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
| `laps` | `race-data-service` | `RaceLap` |
| `car_data` | `race-data-service` | `RaceTelemetrySample` |
| `location` | `race-data-service` | `RaceLocationSample` |
| `position` | `race-data-service` | `RacePositionSample` |
| `intervals` | `race-data-service` | `RaceIntervalSample` |
| `stints` | `race-data-service` | `RaceStint` |
| `pit` | `race-data-service` | `RacePitStop` |
| `weather` | `race-data-service` | `RaceWeatherSample` |
| `team_radio` | `race-data-service` | `TeamRadioMessage` |
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
| `409` | `SESSION_INGESTION_CONFLICT` | a live job is following this session and the request would write race data (catalog resources and `dispatch: false` are accepted) |
| `413` | `PAYLOAD_TOO_LARGE` | body over 1 MiB |
| `502` | `UPSTREAM_UNAVAILABLE` | every resource failed (nothing ingested) |
| `504` | `UPSTREAM_TIMEOUT` | the request context expired during the run |

## Live jobs

A live job follows one session and stores its data as OpenF1 publishes it. Access terms, account
and tuning: `openf1-live-access.md`. `sessionKey` is the OpenF1 session key (positive integer). The
three routes take no body and answer the same status object.

| Route | Answer |
| --- | --- |
| `POST /providers/openf1/live/{sessionKey}/start` | `202` when the job starts, `200` with the current status when it is already running |
| `POST /providers/openf1/live/{sessionKey}/stop` | `200`; state `stopped`, or `stopping` when the job needs more than 5 s to end; no-op on a finished job |
| `GET /providers/openf1/live/{sessionKey}/status` | `200`, latest job of the session (the 20 latest finished jobs are kept) |

```bash
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:3000/v1/ingestion/providers/openf1/live/9998/start
```

```json
{
  "sessionKey": 9998,
  "sessionId": "openf1:session:9998",
  "meetingKey": 1255,
  "state": "running",
  "sessionStartUtc": "YYYY-MM-DDThh:mm:ssZ",
  "sessionEndUtc": "YYYY-MM-DDThh:mm:ssZ",
  "startedAtUtc": "YYYY-MM-DDThh:mm:ssZ",
  "endedAtUtc": null,
  "consecutiveFailures": 0,
  "catalogIngested": true,
  "resources": [
    {
      "resource": "race_control",
      "rowCount": 12,
      "latestRowAtUtc": "YYYY-MM-DDThh:mm:ssZ",
      "lastSuccessAtUtc": "YYYY-MM-DDThh:mm:ssZ"
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `state` | `waiting` (more than 30 minutes before the session), `running`, `stopping`, then one of `stopped` (on request or service shutdown), `completed` (session over), `failed` |
| `error` | short reason: a failed job (`<resource> incomplete: ...` when a resource was still failing at the session end: ingest it by hand), or `catalog not ingested yet` while the meeting, session and drivers are not all stored |
| `consecutiveFailures` | rounds in a row where every poll failed; during the session an outage is waited out, the job only gives up on refused credentials |
| `resources[].rowCount` | rows sent downstream (a row updated by OpenF1 counts each time) |
| `resources[].error` | short reason of the latest failed poll: `OpenF1 request failed`, `OpenF1 request timed out`, `OpenF1 response too large`, `OpenF1 refused the credentials`, `dispatch to <service> failed` (details in the service log) |

Behaviour:

- A job on a past session stores the whole session, 5 minutes of `car_data` and `location` per
  poll from one hour before the start (OpenF1 keeps the laps to the grid), then completes.
- The session dates are read again every 100 intervals, and every 15 minutes by a waiting job. A
  session still sending `location` or `car_data` past the grace period is followed until the data
  stops, 2 hours more at most.
- The meeting, session and drivers are stored again when the session starts and ends, so the
  session `status` follows (`scheduled`, `live`, `finished`).
- A waiting job counts in `OPENF1_LIVE_MAX_JOBS`.
- Jobs live in memory: after a restart, start the job again, stored rows are not duplicated.
- `overtakes`, results and standings are not live: ingest them by hand after the session.
- A heavy period OpenF1 publishes after it was already passed, empty, for more than a minute is
  not fetched again: a manual ingestion after the session fills it.

| Status | Code | When |
| --- | --- | --- |
| `400` | `VALIDATION_ERROR` | `sessionKey` is not a positive integer |
| `404` | `OPENF1_SESSION_NOT_FOUND` | start: OpenF1 does not know the session |
| `404` | `LIVE_JOB_NOT_FOUND` | stop, status: no job was started for the session |
| `409` | `LIVE_JOB_CONFLICT` | start: `OPENF1_LIVE_MAX_JOBS` jobs are already running, or the job is being stopped (retry) |
| `409` | `SESSION_INGESTION_CONFLICT` | start: a manual ingestion of the session is running |
| `502` | `UPSTREAM_UNAVAILABLE` | start: OpenF1 failed or refused the credentials |
| `503` | `SERVICE_UNAVAILABLE` | start: the service is shutting down |
| `504` | `UPSTREAM_TIMEOUT` | start: OpenF1 did not answer within 20 s |
