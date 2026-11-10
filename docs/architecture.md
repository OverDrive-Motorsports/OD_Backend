# OverDrive Backend — How It Works

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
| `/auth` | `auth-service` (login, register, refresh, sessions) | gateway token |
| `/presets` | `user-data-service` (the user's AR/VR layouts) | **user access token** (JWT) |
| `/v1/championship` | `championship-service` (catalog, standings) | gateway token |
| `/v1/race-data` | `race-data-service` (race, telemetry, replay) | gateway token |
| `/v1/ingestion` | `ingestion-service` (OpenF1 import) | gateway token |

- **`gateway`** is the only thing a client should talk to. It does auth, rate
  limiting, request logging, path/parameter validation, and reverse-proxies to the
  right upstream service, **stripping its prefix** (`/v1/championship/championships`
  reaches championship-service as `/championships`). It has no database and no
  business logic of its own.
- **`auth-service`** owns users and their sessions, and issues the access tokens
  (HS256 JWT, `sub` = user ID, 15 min) and refresh tokens.
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
export TOKEN=$(grep GATEWAY_AUTH_TOKEN .env | cut -d= -f2)
```

## Example 1 — a simple read: `GET /v1/championship/championships`

```bash
curl -H "Authorization: Bearer $TOKEN" $GW/v1/championship/championships
```

1. **Gateway receives the request** on `:3000`. `RateLimit` middleware checks the
   client hasn't exceeded its token bucket. `RequestLogger` wraps the rest of the
   chain to log method/path/status/duration at the end.
2. **`RequireAuthorization`** checks the `Authorization` header against
   `GATEWAY_AUTH_TOKEN`. Missing or wrong → `401` with the shared error envelope
   (`{"error":{"code":"UNAUTHORIZED","status":401,"message":"unauthorized"}}`),
   and the request stops here.
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

## Example 2 — parameter validation: `GET /v1/race-data/sessions/{id}/race/laps`

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$GW/v1/race-data/sessions/openf1:session:9998/race/laps?driverNumber=63"
```

Same auth/rate-limit/logging steps as above, but this prefix (`/v1/race-data`) is
also wrapped with **`ValidateRaceParameters`**
(`gateway/internal/adapters/inbound/http/race_parameter_middleware.go`). It inspects
the path shape and:

- checks `race` is followed by an allow-listed action (`position`, `laps`,
  `stints`, `pitstops`, `control`, `weather`, `radio`, `replay`) — anything else
  gets `400 VALIDATION_ERROR` before ever reaching `race-data-service`;
- checks `driverNumber` and `lapNumber` (path or query) are positive integers.

Only then does the reverse proxy forward `GET
/sessions/openf1:session:9998/race/laps?driverNumber=63` to `race-data-service`.
There, `race_live.controller.go`:

1. checks the session exists by asking `championship-service` (the owner of the
   session catalog) — unknown session → `404 SESSION_NOT_FOUND`, championship-service
   unreachable → `502 UPSTREAM_UNAVAILABLE`;
2. calls the usecase, which queries `RaceLap` rows from `overdrive_race_data` and
   computes `bestLap` / `averageLap` on the fly (they are never stored);
3. answers a single object for one driver, or an array with one object per driver
   when `driverNumber` is omitted. A driver with no lap data → `404 LAP_NOT_FOUND`.

`steebchen/prisma-client-go`'s `FindUnique`/`FindFirst` return `(nil, ErrNotFound)`
on a miss, not `(nil, nil)`: every repository translates that sentinel explicitly
(`isNotFoundErr`) so a missing row becomes a `404`, not a `500`.

## Example 3 — the long-poll: `POST /v1/race-data/sessions/{id}/race/control`

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  "$GW/v1/race-data/sessions/openf1:session:9998/race/control"
```

The gateway keeps the connection open for as long as `race-data-service` takes to
answer. On the service side:

1. The handler asks an in-memory, single-process pub/sub
   (`race_control_broadcaster.go`) to wait for the next race-control event on this
   `sessionId`, bounded to 30 seconds.
2. If a new `race_control` ingestion batch is stored for that session while the
   request is waiting (see Example 4), the broadcaster wakes the handler
   immediately, which responds `200` with the new event(s).
3. If nothing arrives within 30 seconds, the handler responds `200` with an empty
   array `[]` — the client is expected to immediately re-`POST` to keep waiting.
   This is *not* an error; it's the documented long-poll contract.

This mechanism is single-instance by construction (an in-process Go channel, not a
shared broker): with more than one `race-data-service` replica, a client's
long-poll could land on a replica that never sees the event.

## Example 4 — ingestion: `POST /v1/ingestion/providers/openf1/ingestions`

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["race_control"],"dispatch":true}' \
  $GW/v1/ingestion/providers/openf1/ingestions
```

1. Gateway: same auth/rate-limit as always; `/v1/ingestion` has no extra parameter
   validation, so the body is forwarded as-is to `ingestion-service`.
2. `ingestion-service`'s usecase (`ingest_openf1.usecase.go`) calls the OpenF1
   provider adapter to fetch the raw rows of each requested resource (all of them
   when `resources` is omitted), maps them from OpenF1's shape into internal
   `contracts.Dataset`s, and groups the datasets by target service: catalog,
   results and standings go to `championship-service`; laps, telemetry, positions,
   race control, weather, radio, etc. go to `race-data-service`.
3. It **dispatches** each batch over internal HTTP to the target's
   `POST /internal/ingestion/batches`. This endpoint has no authentication of its
   own: it relies on not being reachable from outside (see `docs/security.md`).
4. `race-data-service`'s `AcceptIngestionBatchUseCase` writes the rows. For
   `race_control`, the session's existing `RaceControlEvent` rows are deleted then
   the fresh ones inserted, and the new events are **published** to the broadcaster
   from Example 3, so any client long-polling `POST /race/control` for that session
   wakes up immediately.
5. High-volume datasets (`car_data`/telemetry, `location`, `position`,
   `intervals`, `laps`) are written with up to 16 rows in flight concurrently
   (`runConcurrent` in `ingestion.repository.go`), because the Prisma Go client has
   no bulk `CreateMany` — without this, a full session would take minutes of
   per-row round trips.
6. Parent rows built from a child row's fallbacks are created once and never updated:
   when a `sessions` row arrives before its meeting, championship-service creates a
   placeholder event from it, but a later re-ingestion does not overwrite the real
   event written by `meetings` (otherwise the event name became "Race").
7. The response summarises, per resource, how many rows were fetched and whether
   they were dispatched.

## Example 5 — the replay: catalog then bulk dump

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
telemetry sample **and** one location sample — what the replay needs to animate
cars. That check is a single raw `EXISTS` query on the `sessionId` indexes
(`replay_catalog.repository.go`), because the Prisma Go client has no
`DISTINCT`/`EXISTS` and the sample tables hold millions of rows. Newest first,
`[]` when nothing is playable.

**The replay** is a plain `GET` returning one JSON object. `race_replay_stream.usecase.go`'s
`GetReplay`:

1. validates `sessionId` against `championship-service` (Example 2's pattern);
2. asks `race_stream.repository.go` for **eight** datasets — `telemetry`,
   `position`, `raceControl`, `weather`, `radio`, `pitStop`, `stint`, `lap` — each
   queried **once per call**, sorted by timestamp. Telemetry samples carry no lap
   number nor position: `x`/`y`/`z` come from the nearest `RaceLocationSample` and
   `lapNumber` from the `RaceLap` that started last, both resolved with a
   forward-only pointer over the already-sorted rows. `stint` has no timestamp
   column, so each stint is anchored to the start time of its `lapStart` lap;
3. groups the six driver-scoped datasets by driver number (as a string key) and
   leaves the two track-wide ones (`raceControl`, `weather`) as flat arrays;
4. returns the assembled `domain.RaceReplay` — no pagination, no streaming.

It is a **replay** of already-ingested historical samples, not a live feed. See
`services/race-data-service/doc/endpoint.md` for the full response shape. The
gateway still forwards `Flush()` through its logging middleware
(`statusRecorder.Flush()`, covered by `flush_passthrough_test.go`), so a streaming
endpoint would work through it if one is added.

## Example 6 — per-user data: `POST /presets`

```bash
# 1. log in (gateway token on /auth) and keep the user's access token
JWT=$(curl -sS -X POST $GW/auth/login -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"me@example.com","password":"..."}' | jq -r .token)

# 2. call /presets with the user's token — not the gateway token
curl -X POST $GW/presets -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" -d '{"name":"Race view","data":{"widgets":[]}}'
```

1. The gateway sees the `/presets` prefix, listed in `userTokenRoutePrefixes`
   (`router.go`), and uses **`RequireUserToken`** instead of the gateway-token
   check: the header must carry a valid access token, verified with
   `shared/authjwt` and `AUTH_JWT_SECRET`. The 401 code tells the client what to do:
   `UNAUTHORIZED` (no token), `TOKEN_EXPIRED` (call `/auth/refresh`),
   `INVALID_TOKEN` (log in again). The gateway token is refused here.
2. The prefix is stripped and `POST /` reaches `user-data-service` with the
   `Authorization` header unchanged.
3. `user-data-service` **re-verifies** the token (`requireUser` middleware) and takes
   the user ID **only** from its `sub` claim — no route accepts a user ID from the
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

- Every proxied route requires an `Authorization: Bearer ...` header. `/presets`
  expects the **user's access token**; every other prefix (including `/auth`)
  expects `GATEWAY_AUTH_TOKEN`. `/health` and `/health/{service}` are the only
  routes left open, for infra liveness probes.
- `GATEWAY_AUTH_TOKEN` identifies the client application, not a user. Per-user data
  is only ever protected by the user's access token.
- Rate limiting is an in-memory token bucket per client
  (`GATEWAY_RATE_LIMIT_RPS` / `GATEWAY_RATE_LIMIT_BURST`), reset whenever the
  gateway restarts. Exceeding it returns `429 RATE_LIMIT_EXCEEDED`.
- Apart from user-data-service's token check, none of this exists a second time
  inside the services: they rely on the gateway. Calling a service's port directly
  (e.g. `curl http://localhost:3003/championships`, bypassing `:3000`) skips auth,
  rate limiting and parameter validation. The Compose file publishes those ports for
  local debugging only; they must not be reachable from outside a private network
  in a real deployment (see `docs/security.md`).
