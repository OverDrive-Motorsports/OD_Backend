# OverDrive Security Notes

This document describes the current backend security posture for the active local stack:

- `gateway`
- `auth-service`
- `user-data-service`
- `championship-service`
- `race-data-service`
- `ingestion-service`
- PostgreSQL through Docker Compose

All five services plus the gateway now run under Docker Compose. `auth-service`,
`user-data-service` and the gateway all require `AUTH_JWT_SECRET` in the root `.env` and
refuse to start when it is missing, shorter than 32 bytes or still the template placeholder
(checked once, in `shared/authjwt.ValidateSecret`), so no guessable signing secret can ship by
accident.

## User authentication (access tokens)

Access tokens are HS256 JWTs issued by `auth-service` (`sub` = user ID, `role` = `user` or
`admin`, 15 min expiry). They are verified by a single shared implementation, `shared/authjwt`
(HS256 pinned, `none` and other algorithms rejected, expiry required, non-empty `sub`, unknown
`role` value rejected, missing `role` read as `user`), used by `auth-service`, the gateway and
`user-data-service`.

Access model at the gateway (`gateway/ROUTES.md`):

- **Public**: `/v1/auth` login, register, refresh and OAuth routes need no credential. They are
  an allow-list (any other path under `/v1/auth` needs a user token, so a new route or an
  encoded slash fails closed; encoded dot segments under a public route are left to
  auth-service, whose router redirects them and whose session routes check the token) and are covered by the gateway rate limit.
- **User**: `/v1/championship`, `/v1/race-data`, `/v1/presets` and the rest of `/v1/auth` (the
  session routes) require a
  valid access token of any role. The gateway token is refused there.
- **Admin**: `/v1/ingestion` requires an access token with the `admin` role (`403` for a
  `user` token), or the gateway token.
- `user-data-service` and the auth-service session routes re-verify the token and take the
  user ID **only** from `sub`; no route accepts a user ID from the client. Another user's preset
  or session answers `404` (not `403`) so IDs cannot be probed. Session responses never carry
  the refresh token or its hash.
- The gateway drops any client-supplied `X-User-Id` header. No service trusts such a header: the
  identity always comes from the forwarded token.

### Roles

Every account is created with the `user` role (`User.role`, default `user`). There is no API
to change a role: an operator promotes an account by hand, on the auth database:

```sql
UPDATE "User" SET "role" = 'admin' WHERE "email" = 'operator@example.com';
```

Demotion sets `'user'` back. The role is copied into the access token at login and refresh, so
the change applies at the next refresh, at most 15 minutes later (the lifetime of an access
token already issued).

### Gateway token

`GATEWAY_AUTH_TOKEN` is only for internal or scripted calls (for example an ingestion
trigger run by an operator script or a scheduler). It identifies no user and opens only the
admin routes. It must **never be shipped in a client application**: anything embedded in an
app is public. Clients authenticate with the user's own access token. The gateway refuses to
start with a token shorter than 32 bytes.

**Rotate it when the access model is deployed.** Earlier app builds embedded this token to
call every route, so its current value must be treated as public: generate a new one
(`openssl rand -hex 32`), set it in the deployment's `.env`, restart the gateway, and give it
only to the scripts that trigger ingestion. Until then, anyone holding an old app build can
trigger ingestion.

### CORS

No CORS headers are sent and preflight requests are not answered: the clients are native (AR
platform, mobile app), which do not enforce CORS. This is a decision, not an omission. If a
browser client (web dashboard, WebXR) is ever planned, add CORS handling at the gateway with an
explicit origin allow-list (never `*` with credentials) and `OPTIONS` handled before the method
checks.

> Note: the sections below predate the gateway and describe the services as if they were
> reached directly with no auth or rate limiting. The gateway now enforces mandatory
> bearer-token auth and rate limiting on every proxied route, verify against
> `gateway/internal` before relying on the details further down.

## HTTP Surface

Only the gateway (`3000`) is published on every network interface. The services
(`3001`-`3005`) and Postgres (`5432`) are bound to the host's loopback (`127.0.0.1`) in
`docker-compose.yml`: they stay reachable from the developer's own machine for debugging, but
not from another machine on the network. Services reach each other over the Docker network.

Service ports, reachable from the host only:

| Service | Port | Public local endpoints | Internal endpoints |
| --- | ---: | --- | --- |
| `user-data-service` | `3002` | `/presets` (user JWT required), health | none |
| `championship-service` | `3003` | catalog and health endpoints | `POST /internal/ingestion/batches` |
| `race-data-service` | `3004` | read facade and health endpoints | `POST /internal/ingestion/batches` |
| `ingestion-service` | `3005` | `POST /providers/openf1/ingestions`, live jobs (`/providers/openf1/live/{sessionKey}/...`), health | none |

The `/internal/ingestion/batches` endpoints are service-to-service endpoints. They require the
shared secret `INTERNAL_INGESTION_TOKEN` in the `X-Internal-Token` header, compared in constant
time; a missing or wrong token answers `401` before the body is read. `ingestion-service` sends
it on every dispatch, and the three services refuse to start without a token of at least 32
bytes. Production deployments should additionally keep these services on a private network.

## Baseline Headers

The gateway and every service add these response headers through HTTP middleware. The gateway
sets them first, so its own responses (`401`, `429`, `/health`, proxy errors) carry them too,
and drops the upstream copies of proxied responses to avoid duplicated values:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'`

These headers are intentionally conservative because the services return JSON APIs, not browser-rendered pages.

## Request Size Limits

Request body size is limited before JSON decoding:

| Endpoint class | Limit |
| --- | ---: |
| Public auth routes (login, register, refresh, OAuth) | `16 KiB` |
| Preset create and update | `320 KiB` |
| Manual OpenF1 ingestion request | `1 MiB` |
| Internal ingestion batch dispatch | `256 MiB` |

A body over the limit answers `413 PAYLOAD_TOO_LARGE` with the limit in the message
(`apierror.RequestBody` recognises the `http.MaxBytesError`). The internal batch limit is high
because OpenF1 resources such as `car_data`, `location`, and `intervals` can produce large
normalized payloads; heavy resources are sent one driver per batch to stay well under it.

Responses read from other services are bounded too (`bootstrap.LimitResponse`, an explicit
error instead of a truncated decode): OpenF1 `OPENF1_MAX_RESPONSE_BYTES` (default 128 MiB),
dispatch acknowledgements 1 MiB, championship-service responses read by race-data 32 MiB.

Response sizes are bounded as well: the sample lists are paged (at most 50000 rows per
response), `GET /race/replay` is streamed one driver at a time and at most
`REPLAY_MAX_CONCURRENT` (4) replays are built from the sample tables at once (cached replays, served chunk by chunk, have their own cap
`REPLAY_CACHE_MAX_CONCURRENT`, 32), and the race control long poll holds at
most `RACE_CONTROL_MAX_WAITERS` (2000) waiters, `RACE_CONTROL_MAX_WAITERS_PER_SESSION` (500) per
session; beyond a cap the route answers `503` with `Retry-After`.

## Timeouts

Every HTTP server (gateway and services, `bootstrap.NewServer`) bounds each connection, so a
slow or idle client cannot hold one indefinitely:

| Limit | Value |
| --- | --- |
| `ReadHeaderTimeout` | `5s` |
| `ReadTimeout` (whole request) | `30s` |
| `WriteTimeout` (response) | `60s`, above the 30 s race control long poll |
| `IdleTimeout` (keep-alive) | `120s` |
| `MaxHeaderBytes` | `64 KiB` |

Routes that legitimately take longer extend their own deadline only (`bootstrap.ExtendDeadlines`,
gateway `domain.Route.SlowWrite`): the replay dump 10 minutes, the ingestion trigger 30 minutes,
the internal batch route 5 minutes to read and 15 to answer (only after the internal token check).

On `SIGINT`/`SIGTERM` each binary stops accepting connections, releases the long-poll waiters,
lets in-flight requests finish for up to `SHUTDOWN_TIMEOUT` (default `35s`, above the 30 s long
poll the gateway may be proxying), then disconnects
from its database. Compose runs the binary as PID 1 (not `go run`, which does not forward
the signal) and waits 45 s before killing it.

## Container images

Each binary has a multi-stage `Dockerfile` next to its `go.mod`: a `golang:1.27` stage builds
it (and generates the Prisma client of the services that have one), and the image only holds
the binary on a distroless base: `static-debian12` for the gateway and ingestion-service,
`cc-debian12` for the Prisma services, whose query engine (unpacked into `/tmp` at start) needs
glibc and libssl. Images run as the distroless `nonroot` user (65532), contain no shell, no Go
source and no `.env` file (`.dockerignore` keeps `.env`, `.git` and the generated Prisma clients
out of the build context); configuration comes from the environment only. The container health
check runs the binary's own `healthcheck` command. CI builds every image and fails when it runs
as another user or contains `.go`, `go.mod` or `.env` files. Base images are pinned by tag and
updated by hand. The `*-dbsync` jobs (`db push`) and `docker-compose.dev.yml` are the
development exceptions: they run the mounted sources in `golang:1.27`, for local use only.

`ingestion-service` also has outbound timeouts:

| Variable | Purpose | Default |
| --- | --- | --- |
| `OPENF1_TIMEOUT` | OpenF1 provider request timeout | `10s` (`30s` in Compose) |
| `OPENF1_RETRY_COUNT` | OpenF1 retry count (5xx, 408, network errors) | `2` |
| `OPENF1_RETRY_DELAY` | Delay between OpenF1 retries | `800ms` |
| `OPENF1_MIN_REQUEST_INTERVAL` | Minimum time between two OpenF1 requests | `350ms` |
| `OPENF1_MAX_RESPONSE_BYTES` | Maximum size of one OpenF1 response | `134217728` (128 MiB) |
| `DISPATCH_TIMEOUT` | Downstream batch dispatch timeout | `10m` |

Live jobs are bounded too: `OPENF1_LIVE_MAX_JOBS` (default `1`) jobs at once, 2 minutes per poll
or catalog ingestion (retries and dispatch included), an end at the session end plus
`OPENF1_LIVE_GRACE` (at most 2 hours more while the cars still send data), after
`OPENF1_LIVE_MAX_FAILURES` failed rounds in a row once the session and its grace period are over
or when OpenF1 refuses the credentials (an outage during the session is waited out), and a stop
with the service. A resource that
keeps failing does not hold the job past the session.

`DISPATCH_TIMEOUT` is intentionally longer because downstream Prisma writes can take minutes for large OpenF1 datasets.

## Secrets And Local Config

Local `.env` files are ignored by git. Templates (`.env.template` at the root, and the
per-service `.env.template` used to run a single service outside Docker) contain no secret
value: every secret is empty and must be generated, e.g. `openssl rand -hex 32`. Host-side
Prisma tools (`tools/prisma.sh`) build their database URL from `POSTGRES_USER` /
`POSTGRES_PASSWORD` in the root `.env`, which they parse without executing it; no credential is
hard-coded.

| Secret | Used by | Rule |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | Postgres, every database URL | required (the Postgres image refuses an empty password) |
| `GATEWAY_AUTH_TOKEN` | gateway | at least 32 bytes, compared in constant time; opens the admin routes only, never shipped in a client; rotated when the access model is deployed |
| `AUTH_JWT_SECRET` | auth-service, gateway, user-data-service | at least 32 bytes, not a placeholder |
| `INTERNAL_INGESTION_TOKEN` | ingestion-service, championship-service, race-data-service | at least 32 bytes |
| `OPENF1_USERNAME` / `OPENF1_PASSWORD` | ingestion-service | optional, both or neither; only sent to `OPENF1_TOKEN_URL` (https required, as for `OPENF1_BASE_URL` with an account; redirects refused), exchanged for a one hour token; never logged |

`docker-compose.yml` reads Postgres and gateway credentials (`POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_DB`, `GATEWAY_AUTH_TOKEN`, ...) from a root `.env`
file via variable interpolation, with no hardcoded values or fallback defaults in
the compose file itself, see `.env.template` for the expected keys. This keeps
actual values out of version control, even for local development ones.

These are development-grade values only. Production should source database and
gateway credentials from the deployment secret manager, not from a checked-in
`.env.template`-style file.

## Toolchain And Dependencies

- Go 1.27 everywhere (Compose images, CI, `go.work` and every `go.mod`): only the two latest Go
  releases receive security fixes, so the toolchain must follow them.
- The CI `lint` job runs `govulncheck` (reachable known vulnerabilities, standard library
  included), `staticcheck` and `gosec` on every module and fails on any finding. An accepted
  finding is annotated inline with its reason (`//lint:ignore`, `// #nosec <rule> -- <reason>`).
- Dependencies, GitHub Actions and Compose images are updated by hand (no update bot); every
  action is pinned by commit SHA. The Postgres image is pinned to a minor version; a major
  upgrade needs a dump/restore.
- `go.mongodb.org/mongo-driver/v2` is only pulled by prisma-client-go for its BSON decoder; it is
  kept on its latest version until the Prisma client is replaced.

## Known Gaps

- This document predates the API gateway (`gateway/`), which now exists and is fully implemented. The gateway enforces the access model described above on every proxied route and applies in-memory token-bucket rate limiting; `/health` and `/health/{service}` are intentionally left open for infra probes. See `gateway/ROUTES.md` for current behavior, don't trust the bullet points below at face value.
- The public auth routes rely on the global gateway rate limit only: there are no dedicated per-account or per-IP limits on login, registration and refresh yet.
- Individual services behind the gateway (`championship-service`, `race-data-service`, etc.) have no authentication of their own on their public routes: they rely on the gateway, and on their ports being bound to loopback. The internal ingestion endpoints are protected by `INTERNAL_INGESTION_TOKEN`.
- No rate limiting is implemented inside the individual Go services themselves (only at the gateway edge). The gateway keys its limiter on the TCP peer address; `X-Forwarded-For` is only read when the peer is listed in `GATEWAY_TRUSTED_PROXIES`, from the right, so a client cannot pick a new identity per request. Idle buckets are purged.
- The gateway reverse proxy still uses the `httputil.ReverseProxy.Director` hook, deprecated since Go 1.26 (accepted in staticcheck). Moving to `Rewrite` changes the `X-Forwarded-*` headers sent upstream and is tracked separately.

These gaps should be addressed before exposing the services beyond a local or private development network.
