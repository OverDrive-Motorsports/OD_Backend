# championship-service endpoints

Through the gateway the public routes are under `/v1/championship` and require the user's
access token (`Authorization: Bearer <access token>`, any role); the gateway token is refused.
Without a token: `401 UNAUTHORIZED`, expired: `401 TOKEN_EXPIRED`, otherwise invalid:
`401 INVALID_TOKEN` (see `gateway/ROUTES.md`). The service itself checks no user token: it
relies on the gateway.

## `GET /health`

Returns the runtime status of the service.

## `POST /internal/ingestion/batches`

Internal endpoint used by `ingestion-service` to push normalized catalog and standings datasets.

Requires the header `X-Internal-Token: <INTERNAL_INGESTION_TOKEN>` (the shared secret of the
root `.env`, compared in constant time). Missing or wrong token: `401`
`{"error":{"code":"UNAUTHORIZED","status":401,"message":"invalid internal token"}}`, returned
before the body is read. The service refuses to start without a token of at least 32 bytes.

The body is limited to 256 MiB (larger: `413 PAYLOAD_TOO_LARGE`). The route may read its body
for up to 5 minutes and answer within 15 minutes, the server defaults being 30 s and 60 s.

Accepted dataset names:

- `event_catalog`
- `session_catalog`
- `driver_catalog`
- `session_result`
- `starting_grid`
- `driver_championship_standings`
- `team_championship_standings`

## Public catalog endpoints

All responses are **camelCase** JSON. List endpoints return **bare JSON arrays**
(`[...]`), never a `{ "count", "data" }` envelope.

- `GET /championships`
- `GET /championships/{code}/events` (query: `season`)
- `GET /championships/{code}/sessions` every session of the championship flattened with its event context, oldest first: `[{ "sessionId", "name", "type", "status", "startTime", "endTime", "eventId", "eventName", "seasonYear", "circuit" }]`. `[]` when the championship has no session, `404` `CHAMPIONSHIP_NOT_FOUND` for an unknown code. Used by race-data-service's replay catalog.
- `GET /events/{eventId}`
- `GET /events/{eventId}/sessions` (query: `type`)
- `GET /sessions/{sessionId}`
- `GET /sessions/{sessionId}/drivers` (query: `teamId`) - each driver carries `driverPicture` (headshot URL from OpenF1 `headshot_url`) when the provider supplied one
- `GET /sessions/{sessionId}/teams`
- `GET /sessions/{sessionId}/datasets/{dataset}`
- `GET /sessions/{sessionId}/standings` (query: `driverNumber`) - generic standings (currently backed by session race results)
- `GET /sessions/{sessionId}/standings/race` - deprecated alias of `/standings`, kept for backward compatibility with existing internal consumers
- `GET /drivers/{driverNumber}/profile` (query: `championshipCode`) - global, session-independent driver profile, including `driverPicture` when available

### Standings rows

```json
[
  { "position": 1,    "driverNumber": 63, "teamId": "openf1:f1:team:mercedes", "status": "finished", "gapToLeader": "0", "points": 25 },
  { "position": null, "driverNumber": 16, "teamId": "openf1:f1:team:ferrari",  "status": "dnf" }
]
```

- `position` is `null` for a driver the provider did not classify (typically
  DNS / DSQ, or a DNF short of the classification distance) - never `0`. A DNF
  that is still classified keeps its position and has `status: "dnf"`.
- `status` is `finished`, `dnf`, `dns` or `dsq`, derived from OpenF1's
  `dsq` / `dns` / `dnf` flags (most severe first); omitted when the provider
  gave neither a flag nor a position.
- Order: classified drivers by position, then non-classified drivers by driver
  number.

### Weather

This service owns no weather data, so session payloads carry no weather field.
The weather at the start of a session is the first sample of
`GET /v1/race-data/sessions/{sessionId}/race/weather` (bare array, oldest first).

Supported session datasets:

- `session_result`
- `starting_grid`
- `championship_drivers`
- `championship_teams`

`starting_grid` may be sourced from the relevant qualifying session when OpenF1 does not expose grid data directly on the Race session. It is still persisted against the requested Race session ID.

### Example

```bash
curl http://localhost:3003/championships
curl http://localhost:3003/championships/f1/events
curl http://localhost:3003/events/<EVENT_ID>/sessions
curl http://localhost:3003/sessions/<SESSION_ID>/datasets/session_result
curl http://localhost:3003/sessions/<SESSION_ID>/datasets/starting_grid
```
