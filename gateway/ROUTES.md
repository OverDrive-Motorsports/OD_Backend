# Gateway Routes

This file lists routes exposed by the gateway and their upstream service mapping.

## Auth Behavior

Every proxied prefix has an access level (`gateway/internal/config/routes_*.go`). `/health` and
`/health/{service}` stay open for infra probes.

| Access | Prefixes | Expected `Authorization` |
| --- | --- | --- |
| Public | `/v1/auth/login`, `/v1/auth/register`, `/v1/auth/refresh`, `/v1/auth/oauth/...` (and the same under `/auth`) | none |
| User | the rest of `/v1/auth` (the session routes) and of `/auth`, `/v1/presets` (and `/presets`), `/v1/championship` and `/v1/race-data` (except `PUT`) | `Bearer <access token>`, the JWT returned by login, register, OAuth or refresh, verified with `AUTH_JWT_SECRET`. Any role. The gateway token is **refused**. |
| Admin | `/v1/ingestion`, and every `PUT` on `/v1/championship` and `/v1/race-data` (video feed writes) | `Bearer <access token>` whose `role` claim is `admin`, or `Bearer <GATEWAY_AUTH_TOKEN>` for internal and scripted calls |

Rejections carry the code that tells the client what to do, the same codes the services answer
themselves:

| Status | Code | When | Client action |
| --- | --- | --- | --- |
| `401` | `UNAUTHORIZED` | no `Authorization: Bearer ...` header | log in |
| `401` | `TOKEN_EXPIRED` | access token past its `exp` | call `POST /v1/auth/refresh` |
| `401` | `INVALID_TOKEN` | any other invalid token, including the gateway token on a user route | log in again |
| `403` | `FORBIDDEN` | valid access token without the `admin` role on an admin route | none |

The `Authorization` header is forwarded unchanged: `user-data-service` and the auth-service
session routes re-verify the JWT and take the user ID from it. The access token lives 15
minutes and its role is read from the database at each login and refresh, so a promotion or
demotion applies at the next refresh at the latest.

Error body (shared `apierror` envelope):

```json
{
  "error": { "code": "TOKEN_EXPIRED", "status": 401, "message": "token expired" }
}
```

### Versioning and transition

`/auth` and `/presets` are also exposed as `/v1/auth` and `/v1/presets`, with the same
upstream routes and rules. The unversioned paths stay during the transition announced to the
clients and will then be removed; new clients use `/v1/...` only.

### CORS

The gateway sends no CORS headers and does not answer CORS preflights: the clients are the
native AR and mobile apps. A browser client would need an explicit origin allow-list at the
gateway first (see `docs/security.md`).

## Request Headers

Client headers accepted by gateway:

- `Authorization`: `Bearer <access token>` on user routes, `Bearer <access token with the admin role>` or `Bearer <GATEWAY_AUTH_TOKEN>` on admin routes, none on public routes (see Auth Behavior)
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

| Domain | Gateway Prefix | Service env target | Path rewrite | Access |
| --- | --- | --- | --- | --- |
| Auth, public | `/v1/auth/{login,register,refresh,oauth}`, same under `/auth` | `AUTH_SERVICE_URLS` | prefix stripped (`/v1/auth/login` reaches `/login`) | public |
| Auth, rest (sessions) | `/v1/auth`, `/auth` | `AUTH_SERVICE_URLS` | prefix stripped | user |
| User data | `/v1/presets`, `/presets` | `USER_DATA_SERVICE_URLS` | replaced by `/presets` (the service routes live under `/presets`) | user |
| Championship v1 facade | `/v1/championship` | `CHAMPIONSHIP_SERVICE_URLS` | prefix stripped | user |
| Race data v1 facade | `/v1/race-data` | `RACE_DATA_SERVICE_URLS` | prefix stripped | user |
| Ingestion | `/v1/ingestion` | `INGESTION_SERVICE_URLS` | prefix stripped | admin |

## Championship V1 Public Endpoints

All routes below are exposed by gateway under `/v1/championship/*` and proxied to `championship-service`.

Response bodies are camelCase JSON; list endpoints return bare arrays (`[...]`), never a
`{ "count", "data" }` envelope.

### Catalog

| Gateway | Upstream |
| --- | --- |
| `GET /v1/championship/championships` | `GET /championships` |
| `GET /v1/championship/championships/{code}/events` | `GET /championships/{code}/events` (query: `season`) |
| `GET /v1/championship/championships/{code}/sessions` | `GET /championships/{code}/sessions` (every session with its event context) |
| `GET /v1/championship/events/{eventId}` | `GET /events/{eventId}` |
| `GET /v1/championship/events/{eventId}/sessions` | `GET /events/{eventId}/sessions` (query: `type`) |

### Session

| Gateway | Upstream |
| --- | --- |
| `GET /v1/championship/sessions/{sessionId}` | `GET /sessions/{sessionId}` |
| `GET /v1/championship/sessions/{sessionId}/drivers` | `GET /sessions/{sessionId}/drivers` (query: `teamId`) |
| `GET /v1/championship/sessions/{sessionId}/teams` | `GET /sessions/{sessionId}/teams` |
| `GET /v1/championship/sessions/{sessionId}/broadcast` | `GET /sessions/{sessionId}/broadcast` (`{ sessionId, feeds }`) |
| `PUT /v1/championship/sessions/{sessionId}/broadcast` | `PUT /sessions/{sessionId}/broadcast` (body `{ "feeds": Feed[] }`, see "Video feed writes") |
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
| `GET /v1/race-data/championships/{code}/replays` | `GET /championships/{code}/replays` (playable replays, newest first, see race-data `doc/endpoint.md`) |
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
| `GET /v1/race-data/sessions/{sessionId}/datasets/{dataset}` | `GET /sessions/{sessionId}/datasets/{dataset}` (query: `limit`, `offset`, see Pagination below) |
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
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/broadcast` | `GET /sessions/{sessionId}/drivers/{driverNumber}/broadcast` (`{ sessionId, driverNumber, feeds }`) |
| `PUT /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/broadcast` | `PUT /sessions/{sessionId}/drivers/{driverNumber}/broadcast` (body `{ "feeds": Feed[] }`, see "Video feed writes") |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/{dataset}` | `GET /sessions/{sessionId}/drivers/{driverNumber}/{dataset}` (query: `limit`, `offset`) |
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

### Live Race (public contract, camelCase, validated)

All routes below are camelCase, bare-array-or-object JSON
(no `{ "count", "data" }` envelope). `POST /race/control` is a long-poll: the request
blocks until a new race control event batch is available (30 s at most, then `[]`),
then closes, the client must re-POST to keep receiving events. It works with several
race-data-service instances. When an instance already holds its maximum number of waiting
clients it answers `503 SERVICE_UNAVAILABLE` with `Retry-After`: retry after that delay.

| Gateway | Upstream |
| --- | --- |
| `GET /v1/race-data/sessions/{sessionId}/race/position` | `GET /sessions/{sessionId}/race/position` (query: `driverNumber`, `lapNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/race/laps` | `GET /sessions/{sessionId}/race/laps` (query: `driverNumber`, `lapNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/race/stints` | `GET /sessions/{sessionId}/race/stints` (query: `driverNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/race/pitstops` | `GET /sessions/{sessionId}/race/pitstops` (query: `driverNumber`) |
| `POST /v1/race-data/sessions/{sessionId}/race/control` | `POST /sessions/{sessionId}/race/control` (long-poll) |
| `GET /v1/race-data/sessions/{sessionId}/race/weather` | `GET /sessions/{sessionId}/race/weather` |
| `GET /v1/race-data/sessions/{sessionId}/race/radio` | `GET /sessions/{sessionId}/race/radio` (query: `driverNumber`) |
| `GET /v1/race-data/sessions/{sessionId}/race/replay` | `GET /sessions/{sessionId}/race/replay` (query: `driverNumber`, `fromUtc`, `toUtc`), see below |

#### `GET /sessions/{sessionId}/race/replay` (plain JSON bulk dump)

A plain JSON bulk dump of every stored sample for a session across eight
datasets, `telemetry`, `position`, `raceControl`, `weather`, `radio`,
`pitStop`, `stint`, `lap`, returned as one response body. Additive, not a
replacement for the routes above. It replays already-ingested historical
data for a finished/stored session; there is no pacing, it's one `GET`
returning the whole history. The body is streamed (same JSON shape as a single
object), so a client should read it incrementally; the gateway lets this route
take up to 10 minutes instead of its 60 s default. `fromUtc`/`toUtc` (RFC 3339,
inclusive) restrict every dataset to a time window. When too many replays are
being built from the database at once, the route answers `503 SERVICE_UNAVAILABLE`
with `Retry-After` (cached replays have a separate, larger cap). Driver-scoped datasets
(`telemetry`, `position`, `radio`, `pitStop`, `stint`, `lap`) are objects
keyed by driver number (string); `weather`/`raceControl` are always
track-wide flat arrays (never filtered by `driverNumber`). `stint` entries
are sorted by an **approximated** timestamp (anchored to the `lapStart`
lap's recorded start time, `RaceStint` has no timestamp column at all) and
`lap` entries are ordered by lap **completion**
(`dateStartUtc + lapDurationSec`), not lap start, see
`services/race-data-service/doc/endpoint.md` for the full per-dataset
breakdown and payload shapes.

Note: this route used to be a Server-Sent Events stream
(`/race/replay/stream`, paced by a `speed` query param). It was replaced by
this plain-JSON bulk dump, no more SSE, pacing, or `speed` param. The
gateway's SSE flush-passthrough fix on `statusRecorder`
(`logging_middleware.go`'s explicit `Flush()` forwarding, see below) and its
regression test (`flush_passthrough_test.go`) were left in place even though no
current route uses SSE, since they're harmless and directly reusable if a
streaming endpoint is added again in the future.

#### SSE flush-passthrough (kept for future streaming endpoints)

Go's `httputil.ReverseProxy` auto-detects `Content-Type: text/event-stream`
(and any response with unknown `Content-Length`) and flushes immediately
regardless of `FlushInterval`, no gateway config change is needed for that
part. However, `RequestLogger`'s `statusRecorder` (`logging_middleware.go`)
wraps the response writer to capture the status code, and embedding the
`http.ResponseWriter` **interface** only promotes the methods declared on
that interface (`Header`/`Write`/`WriteHeader`), it does **not** promote
`Flush()`, which belongs to the separate `http.Flusher` interface, even
though the concrete writer underneath implements it. Left unfixed, this
silently breaks SSE through the gateway: frames buffer until the whole
response completes instead of arriving incrementally, even though hitting
the upstream service directly works fine. `statusRecorder` has an explicit
`Flush()` that type-asserts and forwards to the underlying writer. **Any
future ResponseWriter wrapper added to this package's middleware chain needs
the same explicit forwarding** (`Flush`, and `Hijack`/`Push`/etc. if ever
needed), this is not automatic from Go's embedding rules. Verified with
`gateway/internal/adapters/inbound/http/flush_passthrough_test.go`, which proxies a
paced fake SSE upstream through the full real middleware chain
(`RequestLogger` → `RateLimit` → auth → `ValidateRaceParameters` →
`ReverseProxy`) and asserts the client observes the same inter-frame delay
the upstream introduced, rather than everything arriving at once at the end, useful as a template if a streaming route is added again.

### Live Telemetry (public contract, camelCase, validated)

`battery` is intentionally absent from `/telemetry/engine` (no data source).

| Gateway | Upstream |
| --- | --- |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/speed` | `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/speed` (query: `lapNumber`, `limit`, `offset`) |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/engine` | `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/engine` (query: `lapNumber`, `limit`, `offset`) |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/location` | `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/location` (query: `lapNumber`, `limit`, `offset`) |
| `GET /v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/intervals` | `GET /sessions/{sessionId}/drivers/{driverNumber}/telemetry/intervals` |

### Pagination

The sample lists (session and driver `{dataset}` routes, `/telemetry/speed`, `/engine`,
`/location`) return at most `limit` rows (`1` to `50000`, default `50000`) from `offset`
(default `0`). When more rows exist, the response has an `X-Next-Offset` header holding the
offset of the next page; without it, the page is the last one. The JSON shape is unchanged; in
the `{dataset}` envelopes, `count` is the number of rows of this page, not the total. A list of
more than 50000 rows requested without paging is cut at 50000 and only `X-Next-Offset` tells it:
clients reading session-wide sample lists must follow it.

### WebSocket Live

| Gateway | Upstream |
| --- | --- |
| `WS /v1/race-data/live?sessionId={sessionId}` | `WS /live?sessionId={sessionId}` |

### Video feed writes (the only `PUT` routes)

The gateway gates HTTP methods at the edge, before proxying:

| Prefix | Allowed methods |
| --- | --- |
| `/v1/championship/*` | `GET` everywhere; `PUT` **only** on `/v1/championship/sessions/{sessionId}/broadcast` |
| `/v1/race-data/*` | `GET` and `POST` everywhere; `PUT` **only** on `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/broadcast` |

Every other method/path combination (any `PUT` elsewhere, `DELETE`, `PATCH`, ...)
is answered by the gateway with `405` once the token is accepted
`{ "error": { "code": "METHOD_NOT_ALLOWED", "status": 405, "message": "method not allowed" } }`.
Feeds are shared by every user, so any `PUT` on these two prefixes is an **admin**
call (see Auth Behavior): an access token with the `admin` role or the gateway
token; a `user` token gets `403 FORBIDDEN`. Reading feeds stays a user call.
`driverNumber` is still validated as a positive integer on the race-data route.

Both routes take `{ "feeds": Feed[] }` and replace the whole list. A `Feed` is a
stable descriptor, never a playable DRM URL or a user token:

```json
{ "provider": "f1tv",    "contentId": "1000005432", "channelId": "1017", "label": "Onboard" }
{ "provider": "youtube", "url": "https://www.youtube.com/watch?v=...", "label": "Highlights" }
{ "provider": "hls",     "url": "https://cdn.example.com/demo/index.m3u8", "label": "Demo" }
```

Validation (enforced by the upstream services, answered as `400 VALIDATION_ERROR`
naming `feeds[<index>].<field>`): `provider` in `f1tv | youtube | hls`;
`contentId` + `channelId` required for `f1tv` (non-empty, at most 64 chars) and
forbidden otherwise; `url` required for `youtube` / `hls` (absolute `https`, at
most 2048 chars, YouTube host `youtube.com` / `www.youtube.com` /
`m.youtube.com` / `youtu.be`, HLS path ending in `.m3u8`) and forbidden for
`f1tv`; `label` optional, at most 80 chars; at most 20 feeds; no duplicates.
Unknown session → `404 SESSION_NOT_FOUND`; driver not in the session (race-data
route) → `404 DRIVER_NOT_FOUND`. The `GET` counterparts always return `feeds`
as an array (`[]` when empty), never `null`. Full request/response details:
`services/championship-service/doc/endpoint.md` and
`services/race-data-service/doc/endpoint.md`.

Frontend resolution, for reference: `youtube` / `hls` URLs are played directly;
`f1tv` feeds are resolved on the device with the user's own F1 TV account
(token never sent to OverDrive) from `contentId` + `channelId`; users without an
account skip them.

### Parameter Validation

Gateway returns `400` with `{ "error": "invalid parameter" }` when one of these values is invalid on `/v1/race-data/*`:

- `dataset` for `/sessions/{sessionId}/datasets/{dataset}`
- `segment` for `/drivers/{driverNumber}/{segment}` (allowed: driver datasets + `profile` + `broadcast`)
- `action` for `/sessions/{sessionId}/race/{action}` (allowed: `position`, `laps`, `stints`, `pitstops`, `control`, `weather`, `radio`, `replay`)
- `action` for `/sessions/{sessionId}/drivers/{driverNumber}/telemetry/{action}` (allowed: `speed`, `engine`, `location`, `intervals`)
- `driverNumber` when present on a driver path or as a `driverNumber` query parameter (must be a positive integer)
- `lapNumber` on `/drivers/{driverNumber}/laps/{lapNumber}/location` or as a `lapNumber` query parameter (must be a positive integer)

Gateway also validates `driverNumber` on `/v1/championship/drivers/{driverNumber}/profile` and
any `driverNumber` query parameter on `/v1/championship/*` as a positive integer.

## Error Handling Notes

- Upstream service responses are passed through as-is (including status code and body).
- Gateway-generated errors include:
  - `401` missing, expired or invalid token on a user or admin route (see Auth Behavior)
  - `403` valid access token without the `admin` role on `/v1/ingestion`
  - `429` rate limit exceeded
  - `400` invalid championship dataset/driverNumber parameter on v1 championship routes
  - `400` invalid race v1 parameter (`dataset`, `action`, `driverNumber`, `lapNumber`)
  - `405` method not allowed on `/v1/championship/*` (anything but `GET`, except `PUT` on the session broadcast route) and `/v1/race-data/*` (anything but `GET`/`POST`, except `PUT` on the driver broadcast route)
  - `502` upstream unavailable/proxy failure
- Upstream limits passed through: `413 PAYLOAD_TOO_LARGE` for a body over a route's size limit
  (auth 16 KiB, presets 320 KiB, feed lists 64 KiB, ingestion trigger 1 MiB), `503 SERVICE_UNAVAILABLE` with
  `Retry-After` when race control waiters or concurrent replays are at their cap.

## Notes

- Gateway target URLs are read from `gateway/.env` (or process env vars), not from hardcoded defaults.
- Multiple URLs in one `*_SERVICE_URLS` variable are load-balanced with round-robin.
- Rate limiting is currently in-memory; counters reset when gateway restarts.
- Connection limits: headers within 5 s, request read within 30 s, response written within
  60 s (enough for the 30 s race control long poll), idle keep-alive closed after 120 s,
  headers at most 64 KiB. Two routes get a longer write deadline (`domain.Route.SlowWrite`):
  `/v1/race-data/.../race/replay` (10 minutes) and `/v1/ingestion` (30 minutes).
- On `SIGTERM` the gateway and every service stop accepting connections and let in-flight
  requests finish for up to `SHUTDOWN_TIMEOUT` (default 35 s, above the 30 s long poll the gateway may be proxying); Compose waits 45 s before killing.

## Auth (`/v1/auth`, public routes are an allow-list)

| Gateway route | Upstream route (`auth-service`) | Access |
| --- | --- | --- |
| `POST /v1/auth/register` | `POST /register` | public |
| `POST /v1/auth/login` | `POST /login` | public |
| `POST /v1/auth/refresh` | `POST /refresh` | public |
| `GET /v1/auth/oauth/providers` | `GET /oauth/providers` | public |
| `GET /v1/auth/oauth/providers/{providerId}/authorize` | `GET /oauth/providers/{providerId}/authorize` | public |
| `POST /v1/auth/oauth/{providerId}/login` | `POST /oauth/{providerId}/login` | public |
| `POST /v1/auth/oauth/{providerId}/register` | `POST /oauth/{providerId}/register` | public |
| `GET /v1/auth/sessions` | `GET /sessions` | user |
| `GET /v1/auth/sessions/{sessionId}` | `GET /sessions/{sessionId}` | user |
| `DELETE /v1/auth/sessions/{sessionId}` | `DELETE /sessions/{sessionId}` | user |

The same routes answer under `/auth/...` during the transition. The public routes are an
allow-list: any other path under `/v1/auth` (including `/v1/auth/health` or an encoded slash
such as `/v1/auth/sessions%2F...`) needs a user access token. Encoded dot segments under a
public route (`/v1/auth/oauth/%2E%2E/sessions`) are forwarded as-is; auth-service's router
answers them with a redirect and its session routes check the token themselves. Payloads and rules:
`services/auth-service/doc/endpoint.md`.

## User presets (`/v1/presets`, user JWT)

| Gateway route | Upstream route (`user-data-service`) |
| --- | --- |
| `GET /v1/presets` | `GET /presets` |
| `POST /v1/presets` | `POST /presets` |
| `GET /v1/presets/{presetId}` | `GET /presets/{presetId}` |
| `PATCH /v1/presets/{presetId}` | `PATCH /presets/{presetId}` |
| `DELETE /v1/presets/{presetId}` | `DELETE /presets/{presetId}` |

The same routes answer under `/presets/...` during the transition. The prefix is forwarded, so
`GET /v1/presets/health` is a preset lookup (`404 PRESET_NOT_FOUND`), never the service health.
Payloads and rules: `services/user-data-service/doc/endpoint.md`.
