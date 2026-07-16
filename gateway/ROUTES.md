# Gateway Routes

This file lists routes exposed by the gateway and their upstream service mapping.

## Auth Behavior

Bearer auth is **mandatory** on every proxied `/v1/*` route (`/health` and
`/health/{service}` stay open for infra probes):

- no `Authorization` header: `401 Unauthorized`
- `Authorization` header provided but invalid/mismatched: `401 Unauthorized`
- `Authorization: Bearer <GATEWAY_AUTH_TOKEN>` matching the configured token: allowed

Previously, requests with no `Authorization` header at all were let through
unauthenticated — this was a known bypass for a public-facing API and has been fixed.

Gateway returns:

```json
{
  "error": "unauthorized"
}
```

## Request Headers

Client headers accepted by gateway:

- `Authorization` (optional): `Bearer <GATEWAY_AUTH_TOKEN>`
- `Content-Type`: forwarded as received
- `Accept`: forwarded as received

Forwarded/proxy headers added by gateway:

- `X-Forwarded-Host`
- `X-Forwarded-Proto`
- `X-Forwarded-For`

## Health Routes

| Gateway Route | Method | Auth | Target | Behavior |
| --- | --- | --- | --- | --- |
| `/health` | `GET` | None (open) | Gateway | Returns gateway liveness |
| `/health/auth` | `GET` | None (open) | Auth service | Probes auth `/health` |
| `/health/user-data` | `GET` | None (open) | User-data service | Probes user-data `/health` |
| `/health/championship` | `GET` | None (open) | Championship service | Probes championship `/health` |
| `/health/race-data` | `GET` | None (open) | Race-data service | Probes race-data `/health` |
| `/health/ingestion` | `GET` | None (open) | Ingestion service | Probes ingestion `/health` |

## Proxied Prefixes By Domain

| Domain | Gateway Prefix | Service env target | Path rewrite |
| --- | --- | --- | --- |
| Auth | `/auth` | `AUTH_SERVICE_URLS` | prefix stripped |
| User data | `/presets` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| User data | `/providers` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| User data | `/users` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| Championship v1 facade | `/v1/championship` | `CHAMPIONSHIP_SERVICE_URLS` | prefix stripped |
| Race data v1 facade | `/v1/race-data` | `RACE_DATA_SERVICE_URLS` | prefix stripped |
| Ingestion | `/v1/ingestion` | `INGESTION_SERVICE_URLS` | prefix stripped |

## Championship V1 Public Endpoints

All routes below are exposed by gateway under `/v1/championship/*` and proxied to `championship-service`.

Response bodies are camelCase JSON; list endpoints return bare arrays (`[...]`), never a
`{ "count", "data" }` envelope. See `.story/endpoint.md` (validated 2026-07-08) for exact shapes.

### Catalog

| Gateway | Upstream |
| --- | --- |
| `GET /v1/championship/championships` | `GET /championships` |
| `GET /v1/championship/championships/{code}/events` | `GET /championships/{code}/events` (query: `season`) |
| `GET /v1/championship/events/{eventId}` | `GET /events/{eventId}` |
| `GET /v1/championship/events/{eventId}/sessions` | `GET /events/{eventId}/sessions` (query: `type`) |

### Session

| Gateway | Upstream |
| --- | --- |
| `GET /v1/championship/sessions/{sessionId}` | `GET /sessions/{sessionId}` |
| `GET /v1/championship/sessions/{sessionId}/drivers` | `GET /sessions/{sessionId}/drivers` (query: `teamId`) |
| `GET /v1/championship/sessions/{sessionId}/teams` | `GET /sessions/{sessionId}/teams` |
| `GET /v1/championship/sessions/{sessionId}/broadcast` | `GET /sessions/{sessionId}/broadcast` |
| `GET /v1/championship/sessions/{sessionId}/standings` | `GET /sessions/{sessionId}/standings` (query: `driverNumber`) |
| `GET /v1/championship/sessions/{sessionId}/standings/race` | `GET /sessions/{sessionId}/standings/race` (deprecated alias) |
| `GET /v1/championship/drivers/{driverNumber}/profile` | `GET /drivers/{driverNumber}/profile` (query: `championshipCode`) |

### Datasets

| Gateway | Upstream |
| --- | --- |
| `GET /v1/championship/sessions/{sessionId}/datasets/{dataset}` | `GET /sessions/{sessionId}/datasets/{dataset}` |

Gateway validates `{dataset}` against the following list before proxying:

- `session_result`
- `starting_grid`
- `championship_drivers`
- `championship_teams`

If `{dataset}` is not in this list, gateway returns:

- status `400`
- body `{ "error": "invalid parameter" }`

## Race V1 Public Endpoints

All routes below are exposed by gateway under `/v1/race-data/*` and proxied to `race-data-service`.

### Catalog

| Gateway | Upstream |
| --- | --- |
| `GET /v1/race-data/championships` | `GET /championships` |
| `GET /v1/race-data/championships/{code}/events` | `GET /championships/{code}/events` |
| `GET /v1/race-data/events/{eventId}` | `GET /events/{eventId}` |
| `GET /v1/race-data/events/{eventId}/sessions` | `GET /events/{eventId}/sessions` |

### Session Metadata And References

| Gateway | Upstream |
| --- | --- |
| `GET /v1/race-data/sessions/{sessionId}` | `GET /sessions/{sessionId}` |
| `GET /v1/race-data/sessions/{sessionId}/metadata` | `GET /sessions/{sessionId}/metadata` |
| `GET /v1/race-data/sessions/{sessionId}/drivers` | `GET /sessions/{sessionId}/drivers` |
| `GET /v1/race-data/sessions/{sessionId}/teams` | `GET /sessions/{sessionId}/teams` |

### Session Data And Shortcuts

| Gateway | Upstream |
| --- | --- |
| `GET /v1/race-data/sessions/{sessionId}/datasets/{dataset}` | `GET /sessions/{sessionId}/datasets/{dataset}` |
| `GET /v1/race-data/sessions/{sessionId}/standings/race` | `GET /sessions/{sessionId}/standings/race` |
| `GET /v1/race-data/sessions/{sessionId}/broadcast` | `GET /sessions/{sessionId}/broadcast` |
| `GET /v1/race-data/sessions/{sessionId}/weather` | `GET /sessions/{sessionId}/weather` |
| `GET /v1/race-data/sessions/{sessionId}/facts` | `GET /sessions/{sessionId}/facts` |

Gateway validates `{dataset}` for this route against:

- `laps`
- `car_data`
- `telemetry`
- `location`
- `position`
- `intervals`
- `stints`
- `pit`
- `pit_stops`
- `weather`
- `team_radio`
- `overtakes`
- `race_control`
- `session_result`
- `starting_grid`
- `championship_drivers`
- `championship_teams`

### Driver

| Gateway | Upstream |
| --- | --- |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/profile` | `GET /sessions/{sessionId}/drivers/{driverNumber}/profile` |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/broadcast` | `GET /sessions/{sessionId}/drivers/{driverNumber}/broadcast` |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/{dataset}` | `GET /sessions/{sessionId}/drivers/{driverNumber}/{dataset}` |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/laps/{lapNumber}/location` | `GET /sessions/{sessionId}/drivers/{driverNumber}/laps/{lapNumber}/location` |

Supported driver datasets on `/drivers/{driverNumber}/{dataset}`:

- `laps`
- `telemetry`
- `location`
- `position`
- `intervals`
- `stints`
- `pit`
- `radio`
- `result`

### Live Race (public contract, camelCase, validated 2026-07-08)

All routes below are camelCase, bare-array-or-object JSON per `.story/endpoint.md`
(no `{ "count", "data" }` envelope). `POST /race/control` is a long-poll: the request
blocks until a new race control event batch is available (bounded server-side
timeout), then closes — the client must re-POST to keep receiving events.

| Gateway | Upstream |
| --- | --- |
| `GET /v1/race-data/sessions/{sessionId}/race/position` | `GET /sessions/{sessionId}/race/position` (query: `driverNumber`, `lapNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/race/laps` | `GET /sessions/{sessionId}/race/laps` (query: `driverNumber`, `lapNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/race/stints` | `GET /sessions/{sessionId}/race/stints` (query: `driverNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/race/pitstops` | `GET /sessions/{sessionId}/race/pitstops` (query: `driverNumber`) |
| `POST /v1/race-data/sessions/{sessionId}/race/control` | `POST /sessions/{sessionId}/race/control` (long-poll) |
| `GET /v1/race-data/sessions/{sessionId}/race/weather` | `GET /sessions/{sessionId}/race/weather` |
| `GET /v1/race-data/sessions/{sessionId}/race/radio` | `GET /sessions/{sessionId}/race/radio` (query: `driverNumber`) |

### Live Telemetry (public contract, camelCase, validated 2026-07-08)

`battery` is intentionally absent from `/telemetry/engine` (no data source).

| Gateway | Upstream |
| --- | --- |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/speed` | `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/speed` (query: `lapNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/engine` | `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/engine` (query: `lapNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/location` | `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/location` (query: `lapNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/intervals` | `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/intervals` |

### WebSocket Live

| Gateway | Upstream |
| --- | --- |
| `WS /v1/race-data/live?sessionId={sessionId}` | `WS /live?sessionId={sessionId}` |

### Parameter Validation

Gateway returns `400` with `{ "error": "invalid parameter" }` when one of these values is invalid on `/v1/race-data/*`:

- `dataset` for `/sessions/{sessionId}/datasets/{dataset}`
- `segment` for `/drivers/{driverNumber}/{segment}` (allowed: driver datasets + `profile` + `broadcast`)
- `action` for `/sessions/{sessionId}/race/{action}` (allowed: `position`, `laps`, `stints`, `pitstops`, `control`, `weather`, `radio`)
- `action` for `/sessions/{sessionId}/drivers/{driverNumber}/telemetry/{action}` (allowed: `speed`, `engine`, `location`, `intervals`)
- `driverNumber` when present on a driver path or as a `driverNumber` query parameter (must be a positive integer)
- `lapNumber` on `/drivers/{driverNumber}/laps/{lapNumber}/location` or as a `lapNumber` query parameter (must be a positive integer)

Gateway also validates `driverNumber` on `/v1/championship/drivers/{driverNumber}/profile` and
any `driverNumber` query parameter on `/v1/championship/*` as a positive integer.

## Error Handling Notes

- Upstream service responses are passed through as-is (including status code and body).
- Gateway-generated errors include:
  - `401` missing or invalid `Authorization` header (mandatory on every `/v1/*` route)
  - `429` rate limit exceeded
  - `400` invalid championship dataset/driverNumber parameter on v1 championship routes
  - `400` invalid race v1 parameter (`dataset`, `action`, `driverNumber`, `lapNumber`)
  - `502` upstream unavailable/proxy failure

## Notes

- Gateway target URLs are read from `gateway/.env` (or process env vars), not from hardcoded defaults.
- Multiple URLs in one `*_SERVICE_URLS` variable are load-balanced with round-robin.
- Rate limiting is currently in-memory; counters reset when gateway restarts.
