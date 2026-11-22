# OverDrive Backend - How It Works

This document explains how a request actually travels through the system, using a
handful of real `curl` calls as worked examples. For "how do I run this," see
[`getting-started.md`](./getting-started.md).

## The pieces

```
                                   ┌─────────────┐
            client (mobile/AR) ──▶ │   gateway   │  :3000
                                   └──────┬──────┘
                                          │ reverse proxy, per path prefix
       ┌──────────────┬──────────────┬────┴─────────┬──────────────────┐
       ▼              ▼              ▼              ▼                  ▼
  auth-service  user-data-service  championship-  race-data-      ingestion-service
     :3001           :3002         service :3003  service :3004        :3005
       │              │              │              │                  │
       ▼              ▼              ▼              ▼                  ▼
 overdrive_auth  overdrive_user_  overdrive_     overdrive_       (no DB: fetches
   (Postgres)    data (Postgres)  championship   race_data         OpenF1 and pushes
                                  (Postgres)     (Postgres)        into the two above)
```

| Gateway prefix | Service | Auth expected by the gateway |
| --- | --- | --- |
| `/v1/auth/{login,register,refresh,oauth}` | `auth-service` (login, register, refresh, OAuth) | none (public) |
| `/v1/auth` (everything else: the caller's own sessions) | `auth-service` | user access token (JWT) |
| `/v1/presets` | `user-data-service` (the user's AR/VR layouts) | user access token (JWT) |
| `/v1/championship` | `championship-service` (catalog, standings) | user access token (JWT) |
| `/v1/race-data` | `race-data-service` (race, telemetry, replay) | user access token (JWT) |
| `/v1/ingestion` | `ingestion-service` (OpenF1 import) | access token with the `admin` role, or the gateway token |

`/auth` and `/presets` still answer unversioned during the transition to `/v1`.

- **`gateway`** is the only thing a client should talk to. It does auth, rate
  limiting, request logging, path/parameter validation, and reverse-proxies to the
  right upstream service, **stripping its prefix** (`/v1/championship/championships`
  reaches championship-service as `/championships`), except where a route declares an
  upstream prefix (`/v1/presets/...` reaches user-data-service as `/presets/...`).
  It has no database and no business logic of its own.
- **`auth-service`** owns users and their sessions, and issues the access tokens
  (HS256 JWT, `sub` = user ID, `role` = `user` or `admin`, 15 min) and refresh tokens.
- **`user-data-service`** stores per-user data (today: layout presets). It is the only
  service that re-verifies the user's JWT itself and scopes every query to that user.
- **`championship-service`** and **`race-data-service`** each own a private Postgres
  database (via Prisma) and expose a plain HTTP API.
- **`ingestion-service`** has no database. It fetches data from the OpenF1 public
  API, reshapes it, and pushes ("dispatches") it into `championship-service` and
  `race-data-service` over internal HTTP endpoints.
- Each service follows the same hexagonal layout: `core/domain` (entities),
  `core/ports` (interfaces), `core/usecases` (business logic), `adapters/http`
  (HTTP in), `adapters/repository/prisma` (DB out), `adapters/downstream` (calls to
  other services).
- Shared modules: `shared/apierror` (the error envelope every component writes),
  `shared/authjwt` (the single verifier of user access tokens, used by
  auth-service, the gateway and user-data-service), `shared/contracts` (ingestion
  batch format and deterministic IDs), `shared/bootstrap` (config loading).

The examples below assume:

```bash
export GW=http://localhost:3000
# a user access token (15 minutes), from the public login route
export TOKEN=$(curl -sS -X POST $GW/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","password":"..."}' | jq -r .token)
```

## Example 1 - a simple read: `GET /v1/championship/championships`

```bash
curl -H "Authorization: Bearer $TOKEN" $GW/v1/championship/championships
```

1. **Gateway receives the request** on `:3000`. `RateLimit` middleware checks the
   client hasn't exceeded its token bucket. `RequestLogger` wraps the rest of the
   chain to log method/path/status/duration at the end.
2. **`RequireUserToken`** checks the `Authorization` header carries a valid user
   access token (`/v1/championship` has the user access level in
   `routes_championship.go`). A missing token answers `401 UNAUTHORIZED`, an expired one `401
   TOKEN_EXPIRED`, anything else (the gateway token included) `401 INVALID_TOKEN`,
   with the shared error envelope, and the request stops here.
3. The path matches the `/v1/championship` prefix, registered in
   `gateway/internal/config/routes_championship.go` and pointing at
   `CHAMPIONSHIP_SERVICE_URLS`. `ValidateChampionshipDataset` (edge validation for
   dataset names and `driverNumber`) has nothing to check on this path.
4. **`ReverseProxy`** (`gateway/internal/adapters/inbound/http/reverse_proxy.go`)
   strips the `/v1/championship` prefix and forwards `/championships` to the
   upstream, e.g. `http://championship-service:3003/championships`.
5. **`championship-service`** receives `GET /championships` on its own HTTP mux
   (`catalog.routes.go` → `catalog.controller.go`), calls
   `CatalogQueryUseCase.ListChampionships`, which calls the Prisma repository,
   which runs `Championship.FindMany()` against `overdrive_championship`.
6. The controller maps the result to `domain.ChampionshipSummary` (camelCase JSON
   tags) and writes a **bare JSON array**. That response goes back through the
   reverse proxy, unmodified, to the client.

## Example 2 - parameter validation: `GET /v1/race-data/sessions/{id}/race/laps`

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$GW/v1/race-data/sessions/openf1:session:9998/race/laps?driverNumber=63"
```

Same auth/rate-limit/logging steps as above, but this prefix (`/v1/race-data`) is
also wrapped with **`ValidateRaceParameters`**
(`gateway/internal/adapters/inbound/http/race_parameter_middleware.go`). It inspects
the path shape and:

- checks `race` is followed by an allow-listed action (`position`, `laps`,
  `stints`, `pitstops`, `control`, `weather`, `radio`, `replay`) - anything else
  gets `400 VALIDATION_ERROR` before ever reaching `race-data-service`;
- checks `driverNumber` and `lapNumber` (path or query) are positive integers.

Only then does the reverse proxy forward `GET
/sessions/openf1:session:9998/race/laps?driverNumber=63` to `race-data-service`.
There, `race_live.controller.go`:

1. checks the session exists by asking `championship-service` (the owner of the
   session catalog) - unknown session → `404 SESSION_NOT_FOUND`, championship-service
   unreachable → `502 UPSTREAM_UNAVAILABLE`;
2. calls the usecase, which queries `RaceLap` rows from `overdrive_race_data` and
   computes `bestLap` / `averageLap` on the fly (they are never stored);
3. answers a single object for one driver, or an array with one object per driver
   when `driverNumber` is omitted. A driver with no lap data → `404 LAP_NOT_FOUND`.

`steebchen/prisma-client-go`'s `FindUnique`/`FindFirst` return `(nil, ErrNotFound)`
on a miss, not `(nil, nil)`: every repository translates that sentinel explicitly
(`isNotFoundErr`) so a missing row becomes a `404`, not a `500`.

## Example 3 - the long-poll: `POST /v1/race-data/sessions/{id}/race/control`

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  "$GW/v1/race-data/sessions/openf1:session:9998/race/control"
```

The gateway keeps the connection open for as long as `race-data-service` takes to
answer. On the service side:

1. The handler registers as a waiter for the next race-control event on this
   `sessionId`, bounded to 30 seconds. The instance caps its waiters (globally and per
   session); beyond the cap it answers `503` with `Retry-After`.
2. When a `race_control` ingestion batch is stored for that session (see Example 4),
   it is also inserted into the `RaceControlNotification` table. Every
   `race-data-service` instance polls that table each second
   (`shared_race_control_broadcaster.go`) and wakes its local waiters for the session,
   which respond `200` with the new event(s).
3. If nothing arrives within 30 seconds, the handler responds `200` with an empty
   array `[]` - the client is expected to immediately re-`POST` to keep waiting.
   This is *not* an error; it's the documented long-poll contract.

Because the batch goes through Postgres, the long poll works with several replicas: a
client waiting on one replica is woken by a batch ingested through another, at most about a
second later. No broker is involved; the rows are deleted after 5 minutes. On shutdown the
waiters are released at once with `[]` so the client re-POSTs, possibly to another replica.

## Example 4 - ingestion: `POST /v1/ingestion/providers/openf1/ingestions`

```bash
# an admin access token, or GATEWAY_AUTH_TOKEN from a script
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["race_control"],"dispatch":true}' \
  $GW/v1/ingestion/providers/openf1/ingestions
```

1. Gateway: `/v1/ingestion` is admin-only. A `user` access token answers `403
   FORBIDDEN`; an access token with the `admin` role, or the gateway token, passes.
   There is no extra parameter validation, so the body is forwarded as-is to
   `ingestion-service`.
2. `ingestion-service`'s usecase (`ingest_openf1.usecase.go`) calls the OpenF1
   provider adapter to fetch the raw rows of each requested resource (all of them
   when `resources` is omitted), maps them from OpenF1's shape into internal
   `contracts.Dataset`s, and groups the datasets by target service: catalog,
   results and standings go to `championship-service`; laps, telemetry, positions,
   race control, weather, radio, etc. go to `race-data-service`.
3. It **dispatches** each batch over internal HTTP to the target's
   `POST /internal/ingestion/batches`, with the shared `INTERNAL_INGESTION_TOKEN` in the
   `X-Internal-Token` header: without it the receiving service answers `401` before reading
   the body (see `docs/security.md`).
4. `race-data-service`'s `AcceptIngestionBatchUseCase` writes the rows. For
   `race_control`, the session's existing `RaceControlEvent` rows are deleted then
   the fresh ones inserted, and the new events are **published** to the broadcaster
   from Example 3, so any client long-polling `POST /race/control` for that session
   wakes up within a second, on any instance. Every stored batch also deletes the
   session's cached replays (Example 5).
5. High-volume datasets (`car_data`/telemetry, `location`, `position`,
   `intervals`, `laps`) are written with up to 16 rows in flight concurrently
   (`runConcurrent` in `ingestion.repository.go`), because the Prisma Go client has
   no bulk `CreateMany` - without this, a full session would take minutes of
   per-row round trips.
6. Parent rows built from a child row's fallbacks are created once and never updated:
   when a `sessions` row arrives before its meeting, championship-service creates a
   placeholder event from it, but a later re-ingestion does not overwrite the real
   event written by `meetings` (otherwise the event name became "Race").
7. A full-session request (no `driver_number`) fetches the heavy resources
   (`car_data`, `location`) one driver at a time and dispatches each driver's batch on
   its own, after the light resources: one response or batch never holds a whole
   session. OpenF1 calls are spaced (`OPENF1_MIN_REQUEST_INTERVAL`) and a `429` waits for
   its `Retry-After`.
8. A failing resource (or driver) does not stop the others. The response summarises,
   per resource, its `status` (`ok`, `partial`, `failed`), row count and whether it was
   dispatched; the request fails with `502` only when every resource failed.

## Example 5 - the replay: catalog then bulk dump

```bash
# which sessions can be replayed?
curl -H "Authorization: Bearer $TOKEN" $GW/v1/race-data/championships/f1/replays

# the replay itself
curl -H "Authorization: Bearer $TOKEN" \
  "$GW/v1/race-data/sessions/openf1:session:9998/race/replay?driverNumber=63"
```

**The catalog** (`replay_catalog.*` in race-data-service) lists a championship's
sessions with their event context in one call to championship-service
(`GET /championships/{code}/sessions`), then keeps only those with at least one
telemetry sample **and** one location sample - what the replay needs to animate
cars. That check is a single raw `EXISTS` query on the `sessionId` indexes
(`replay_catalog.repository.go`), because the Prisma Go client has no
`DISTINCT`/`EXISTS` and the sample tables hold millions of rows. Newest first,
`[]` when nothing is playable.

**The replay** is a plain `GET` returning one JSON object, **streamed** so memory stays
bounded whatever the session size. `race_replay_stream.usecase.go`'s `StreamReplay`:

1. the controller validates `sessionId` against `championship-service` (Example 2's
   pattern) and the optional `driverNumber`, `fromUtc`, `toUtc`;
2. a replay slot is taken (at most `REPLAY_MAX_CONCURRENT` at once, `503` beyond);
3. without a time window, a complete cached archive (`RaceArchive` and its
   `RaceDatasetChunk` rows) is served chunk by chunk when one exists;
4. otherwise `race_stream.repository.go` is read **one driver at a time** for each of
   the six driver-scoped datasets (`telemetry`, `position`, `radio`, `pitStop`,
   `stint`, `lap`), then the two track-wide ones (`raceControl`, `weather`), each
   sorted by timestamp. Telemetry samples carry no lap number nor position:
   `x`/`y`/`z` come from the driver's nearest `RaceLocationSample` and `lapNumber`
   from the `RaceLap` that started last, both resolved with a forward-only pointer
   over the already-sorted rows. `stint` has no timestamp column, so each stint is
   anchored to the start time of its `lapStart` lap;
5. each part is written to the response as soon as it is read
   (`replay_json_writer.go`), driver keys in the order a JSON map encoder gives them,
   and a full dump is recorded into the cache at the same time. An ingestion into the
   session deletes its archives, so the cache is never stale.

A failure after the first byte aborts the connection rather than ending a truncated
object.

It is a **replay** of already-ingested historical samples, not a live feed. See
`services/race-data-service/doc/endpoint.md` for the full response shape. The
gateway forwards `Flush()` and exposes `Unwrap()` through its logging middleware
(`statusRecorder`), so the streamed body and the route's longer write deadline pass
through it.

## Example 6 - per-user data: `POST /v1/presets`

```bash
curl -X POST $GW/v1/presets -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" -d '{"name":"Race view","data":{"widgets":[]}}'
```

1. The gateway sees the `/v1/presets` prefix, declared with the user access level
   in `routes_user_data.go`, and uses **`RequireUserToken`**: the header must carry
   a valid access token, verified with `shared/authjwt` and `AUTH_JWT_SECRET`. The
   401 code tells the client what to do: `UNAUTHORIZED` (no token), `TOKEN_EXPIRED`
   (call `/v1/auth/refresh`), `INVALID_TOKEN` (log in again).
2. The route's upstream prefix is `/presets`, so `POST /presets` reaches
   `user-data-service` with the `Authorization` header unchanged. The service
   registers its routes under `/presets`, so no public path can reach its own
   `/health` or any other route.
3. `user-data-service` **re-verifies** the token (`requireUser` middleware) and takes
   the user ID **only** from its `sub` claim - no route accepts a user ID from the
   client. Every repository query is scoped to that user, so another user's preset
   answers `404`, exactly like a missing one.
4. The creation runs in one transaction that starts with a per-user advisory lock
   (`pg_advisory_xact_lock`): the 20-preset limit and the single default preset are
   enforced by the database statement itself, so concurrent requests of one user
   cannot bypass them (details in `services/user-data-service/doc/architecture.md`).

## Identifiers, end to end

Nothing in the public API uses OpenF1's raw numeric keys directly. Every ID is
built deterministically in `shared/contracts/ingestion/identity.go` and reused by
every service, so the same real-world entity always maps to the same string:

| Entity | Format | Example |
| --- | --- | --- |
| Session | `<provider>:session:<session_key>` | `openf1:session:9998` |
| Event | `<provider>:event:<meeting_key>` | `openf1:event:1255` |
| Championship | `<provider>:championship:<code>` | `openf1:championship:f1` |
| Driver | `<provider>:<championshipCode>:driver:<number>` | `openf1:f1:driver:63` |
| Team | `<provider>:<championshipCode>:team:<slug>` | `openf1:f1:team:mercedes` |

User and preset IDs are UUIDs generated by their own service.

## Response contract

The public routes follow the same shape rules:

- JSON field names are **camelCase**.
- Any list-shaped response is a **bare JSON array** (`[...]`), never wrapped in a
  `{"count": ..., "data": [...]}` envelope.
- Every error written by the gateway or a service uses the `shared/apierror`
  envelope: `{"error":{"code":"SESSION_NOT_FOUND","status":404,"message":"session not found"}}`.
  Typical statuses: `400` invalid parameter or body, `401` missing/invalid/expired
  token, `404` unknown resource, `409` conflict (e.g. preset limit), `429` rate
  limited, `502` upstream service unreachable, `504` upstream timeout. The
  `message` never carries internal details.

Known exceptions, kept for now and listed in the backlog:

- race-data-service still serves a legacy facade (`/sessions/{id}`, `/sessions/{id}/datasets/...`,
  `/drivers/{n}/{dataset}`, ...) with snake_case keys and `{count, data}` envelopes;
  new clients should use `/race/*`, `/telemetry/*` and the championship routes.
- A path that matches no route at all answers Go's default plain-text
  `404 page not found`, not the JSON envelope.

## Auth and rate limiting, in one place

- Each proxied prefix declares its access level (`domain.RouteAccess`, in
  `gateway/internal/config/routes_*.go`): **public** (only the `/v1/auth` login,
  register, refresh and OAuth routes, an allow-list), **user** (data, presets, sessions: any valid access token) or
  **admin** (`/v1/ingestion`: access token with the `admin` role, or the gateway
  token). `/health` and `/health/{service}` are also open, for infra liveness probes.
  A route without an access level stops the gateway at startup.
- The `role` claim (`user` or `admin`) is issued by auth-service from `User.role`;
  admins are promoted by hand in SQL (`docs/security.md`).
- `GATEWAY_AUTH_TOKEN` identifies no user: it is for internal and scripted calls,
  opens the admin routes only, and is never shipped in a client.
- Rate limiting is an in-memory token bucket per client
  (`GATEWAY_RATE_LIMIT_RPS` / `GATEWAY_RATE_LIMIT_BURST`), reset whenever the
  gateway restarts. Exceeding it returns `429 RATE_LIMIT_EXCEEDED`.
- Apart from the token checks of user-data-service and of the auth-service session
  routes, none of this exists a second time inside the services: they rely on the gateway. Calling a service's port directly
  (e.g. `curl http://localhost:3003/championships`, bypassing `:3000`) skips auth,
  rate limiting and parameter validation. The Compose file publishes those ports for
  local debugging only; they must not be reachable from outside a private network
  in a real deployment (see `docs/security.md`).
