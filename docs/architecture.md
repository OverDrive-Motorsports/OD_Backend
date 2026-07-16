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
              ┌──────────────────┼──────────────────┐
              ▼                  ▼                   ▼
   championship-service   race-data-service   ingestion-service
        :3003                  :3004               :3005
              │                  │                   │
              ▼                  ▼                   ▼
   overdrive_championship  overdrive_race_data   (no DB of its own,
        (Postgres)             (Postgres)         calls the two above)
```

- **`gateway`** is the only thing a client ever talks to. It does auth, rate
  limiting, request logging, path/parameter validation, and reverse-proxies to the
  right upstream service. It has no database and no business logic of its own.
- **`championship-service`** and **`race-data-service`** each own a private
  Postgres database (via Prisma) and expose their own plain HTTP API — they don't
  know they're behind a gateway.
- **`ingestion-service`** has no database. It fetches data from the OpenF1 public
  API, reshapes it, and pushes ("dispatches") it into `championship-service` and
  `race-data-service` over internal HTTP endpoints.
- Each service follows the same hexagonal layout: `core/domain` (entities),
  `core/ports` (interfaces), `core/usecases` (business logic), `adapters/http`
  (HTTP in), `adapters/repository/prisma` (DB out).

## Example 1 — a simple read: `GET /v1/championship/championships`

```bash
curl -H "Authorization: Bearer admin" http://localhost:3000/v1/championship/championships
```

1. **Gateway receives the request** on `:3000`. `RateLimit` middleware checks the
   client IP hasn't exceeded its token bucket. `RequestLogger` wraps the rest of the
   chain to log method/path/status/duration at the end.
2. **`RequireAuthorization`** checks the `Authorization` header against
   `GATEWAY_AUTH_TOKEN`. Missing or wrong → `401 {"error":"unauthorized"}`,
   request stops here. This applies to every `/v1/*` route, no exceptions.
3. The path matches the `/v1/championship` prefix, registered in
   `gateway/internal/config/routes_championship.go` and pointing at
   `CHAMPIONSHIP_SERVICE_URLS`. Because this prefix has no path-shaped parameters
   here, `ValidateChampionshipDataset` (edge validation for dataset/driverNumber
   params) passes through with nothing to check.
4. **`ReverseProxy`** (`gateway/internal/adapters/inbound/http/reverse_proxy.go`)
   strips the `/v1/championship` prefix and forwards the rest (`/championships`) to
   the upstream, e.g. `http://championship-service:3003/championships`.
5. **`championship-service`** receives `GET /championships` on its own HTTP mux
   (`catalog.routes.go` → `catalog.controller.go`), calls
   `CatalogQueryUseCase.ListChampionships`, which calls the Prisma repository,
   which runs `Championship.FindMany()` against `overdrive_championship`.
6. The controller maps the result to `domain.ChampionshipSummary` (camelCase JSON
   tags) and writes a **bare JSON array** — no `{"count":, "data":[...]}` wrapper.
   That response bubbles back through the reverse proxy, unmodified, to the client.

## Example 2 — parameter validation: `GET /v1/race-data/sessions/{id}/race/laps`

```bash
curl -H "Authorization: Bearer admin" \
  "http://localhost:3000/v1/race-data/sessions/openf1:session:9998/race/laps?driverNumber=63"
```

Same auth/rate-limit/logging steps as above, but this prefix (`/v1/race-data`) is
wrapped with **`ValidateRaceParameters`**
(`gateway/internal/adapters/inbound/http/race_parameter_middleware.go`), which the
`/v1/championship` prefix doesn't get. It inspects the path shape and:

- checks `race` is followed by an allow-listed action (`position`, `laps`,
  `stints`, `pitstops`, `control`, `weather`, `radio`) — an unknown action gets
  `400 {"error":"invalid parameter"}` before ever reaching `race-data-service`;
- checks `driverNumber` (path or query) is a positive integer if present.

Only after that does the reverse proxy forward `GET
/sessions/openf1:session:9998/race/laps?driverNumber=63` to `race-data-service`.
There, `race_live.controller.go` calls a usecase that:

1. queries `RaceLap` rows from `overdrive_race_data` filtered by session/driver,
2. computes `bestLap`/`averageLap` on the fly from those rows (these are never
   stored, only derived at read time),
3. returns them as a single JSON object with a nested `laps` array.

If `openf1:session:9998` doesn't exist at all, the repository's Prisma lookup
returns Prisma's `ErrNotFound` sentinel, which the repository translates into a
clean `(nil, nil)` — the controller then answers `404`, not a raw `500`. (This
translation has to happen explicitly: `steebchen/prisma-client-go`'s
`FindUnique`/`FindFirst` return `(nil, ErrNotFound)` on a miss, not `(nil, nil)`.)

## Example 3 — the long-poll: `POST /v1/race-data/sessions/{id}/race/control`

```bash
curl -X POST -H "Authorization: Bearer admin" \
  "http://localhost:3000/v1/race-data/sessions/openf1:session:9998/race/control"
```

This is the one route where the gateway doesn't just proxy-and-forget: the
underlying `net/http/httputil.ReverseProxy` keeps the connection open for as long
as `race-data-service` takes to answer. On the service side:

1. The handler asks an in-memory, single-process pub/sub
   (`race_control_broadcaster.go`) to wait for the next race-control event on this
   `sessionId`, bounded to 30 seconds.
2. If a new `race_control` ingestion batch is stored for that session while the
   request is waiting (see Example 4), the broadcaster wakes the handler
   immediately, which responds `200` with the new event(s).
3. If nothing arrives within 30 seconds, the handler responds `200` with an empty
   array `[]` — the client is expected to immediately re-`POST` to keep waiting.
   This is *not* an error; it's the documented long-poll contract.

This mechanism is single-instance by construction (it's an in-process Go channel,
not backed by Redis or any external broker) — if `race-data-service` is ever run
as more than one replica, a client's long-poll could land on a replica that never
sees the event, and this would need to move to a shared pub/sub.

## Example 4 — ingestion: `POST /v1/ingestion/providers/openf1/ingestions`

```bash
curl -X POST -H "Authorization: Bearer admin" -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["race_control"],"dispatch":true}' \
  http://localhost:3000/v1/ingestion/providers/openf1/ingestions
```

1. Gateway: same auth/rate-limit as always; `/v1/ingestion` has no extra parameter
   validation middleware (unlike championship/race-data), so the body is forwarded
   as-is to `ingestion-service`.
2. `ingestion-service`'s usecase (`ingest_openf1.usecase.go`) calls the OpenF1
   provider adapter to fetch the raw `race_control` rows for that
   meeting/session, maps them from OpenF1's snake_case shape into an internal
   `contracts.Dataset`, and groups datasets by their target service
   (`shared/contracts/ingestion`).
3. It **dispatches** the batch over plain internal HTTP to `race-data-service`'s
   `POST /internal/ingestion/batches` (this endpoint has no auth of its own — it
   relies entirely on network topology/the gateway not exposing it, see
   `docs/security.md`).
4. `race-data-service`'s `AcceptIngestionBatchUseCase` writes the rows into
   `RaceControlEvent` via the repository (deleting any existing rows for that
   session first, then inserting fresh ones), and — specifically for
   `race_control` batches — also **publishes** the new events to the in-memory
   broadcaster from Example 3, so any client currently long-polling `POST
   /race/control` for that session wakes up immediately instead of waiting out
   the full 30s timeout.
5. High-volume datasets (`car_data`/telemetry, `location`, `position`,
   `intervals`, `laps`) are written with up to 16 rows in flight concurrently
   (`runConcurrent` in `ingestion.repository.go`) instead of one at a time,
   because the underlying Prisma Go client has no bulk `CreateMany` — without
   this, a full session (thousands of samples × ~20 drivers) would take minutes,
   dominated by per-row network round trips rather than any real computation.
6. The HTTP response bubbles back up through `ingestion-service` →
   gateway → client, summarizing what was fetched/mapped/dispatched.

## Identifiers, end to end

Nothing in the public API uses OpenF1's raw numeric keys directly. Every ID is
built deterministically in `shared/contracts/ingestion/identity.go` and reused
by every service, so the same real-world entity always maps to the same string:

| Entity | Format | Example |
| --- | --- | --- |
| Session | `<provider>:session:<session_key>` | `openf1:session:9998` |
| Event | `<provider>:event:<meeting_key>` | `openf1:event:1255` |
| Championship | `<provider>:championship:<code>` | `openf1:championship:f1` |
| Driver | `<provider>:<championshipCode>:driver:<number>` | `openf1:f1:driver:63` |
| Team | `<provider>:<championshipCode>:team:<slug>` | `openf1:f1:team:mercedes` |

## Response contract

Every public route under `/v1/championship/*` and `/v1/race-data/*` follows the
same shape rules (validated 2026-07-08, see `.story/endpoint.md`):

- JSON field names are **camelCase**.
- Any list-shaped response is a **bare JSON array** (`[...]`), never wrapped in a
  `{"count": ..., "data": [...]}` envelope.
- Errors are always `{"error": "<message>"}` with a matching HTTP status —
  `400` for invalid parameters, `401` for missing/invalid auth, `404` for an
  unknown resource, `502` if an upstream service is unreachable.

## Auth and rate limiting, in one place

- Every proxied `/v1/*` route requires `Authorization: Bearer <GATEWAY_AUTH_TOKEN>`
  — missing or wrong both return `401`. `/health` and `/health/{service}` are the
  only routes intentionally left open, for infra liveness probes.
- Rate limiting is an in-memory token bucket per client IP
  (`GATEWAY_RATE_LIMIT_RPS` / `GATEWAY_RATE_LIMIT_BURST`), reset whenever the
  gateway process restarts. Exceeding it returns `429`.
- None of this exists a second time inside the individual services — they trust
  the gateway/network topology entirely. Calling a service's port directly
  (e.g. `curl http://localhost:3003/championships`, bypassing `:3000`) skips auth,
  rate limiting, and parameter validation completely. That's fine for local
  debugging, but it's why those ports shouldn't be reachable from outside a
  private network in a real deployment.
