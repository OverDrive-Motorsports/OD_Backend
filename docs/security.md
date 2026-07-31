# OverDrive Security Notes

This document describes the current backend security posture for the active local stack:

- `championship-service`
- `race-data-service`
- `ingestion-service`
- PostgreSQL through Docker Compose

`auth-service` and `user-data-service` are scaffolded services and are intentionally not part of the active Docker Compose runtime yet.

## HTTP Surface

The active services expose HTTP endpoints directly on local development ports:

| Service | Port | Public local endpoints | Internal endpoints |
| --- | ---: | --- | --- |
| `championship-service` | `3003` | catalog and health endpoints | `POST /internal/ingestion/batches` |
| `race-data-service` | `3004` | read facade and health endpoints | `POST /internal/ingestion/batches` |
| `ingestion-service` | `3005` | `POST /providers/openf1/ingestions`, health | none |

The `/internal/ingestion/batches` endpoints are internal service-to-service endpoints. They are exposed on localhost in Docker Compose for development convenience, but production deployment should put them on a private network or behind gateway policy.

## Baseline Headers

The active services add these response headers through HTTP middleware:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'`

These headers are intentionally conservative because the services return JSON APIs, not browser-rendered pages.

## Request Size Limits

Request body size is limited before JSON decoding:

| Endpoint class | Limit |
| --- | ---: |
| Manual OpenF1 ingestion request | `1 MiB` |
| Internal ingestion batch dispatch | `256 MiB` |

The internal batch limit is high because OpenF1 resources such as `car_data`, `location`, and `intervals` can produce large normalized payloads.

## Timeouts

Each Go HTTP server sets `ReadHeaderTimeout` to protect request header parsing.

`ingestion-service` also has outbound timeouts:

| Variable | Purpose | Default |
| --- | --- | --- |
| `OPENF1_TIMEOUT` | OpenF1 provider request timeout | `10s` |
| `OPENF1_RETRY_COUNT` | OpenF1 retry count | `2` |
| `OPENF1_RETRY_DELAY` | Delay between OpenF1 retries | `800ms` |
| `DISPATCH_TIMEOUT` | Downstream batch dispatch timeout | `10m` |

`DISPATCH_TIMEOUT` is intentionally longer because downstream Prisma writes can take minutes for large OpenF1 datasets.

## Secrets And Local Config

Local `.env` files are ignored by git. Templates should contain only non-secret placeholders.

`docker-compose.yml` reads Postgres and gateway credentials (`POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_DB`, `GATEWAY_AUTH_TOKEN`, ...) from a root `.env`
file via variable interpolation, with no hardcoded values or fallback defaults in
the compose file itself — see `.env.template` for the expected keys. This keeps
actual values out of version control, even for local development ones.

These are development-grade values only. Production should source database and
gateway credentials from the deployment secret manager, not from a checked-in
`.env.template`-style file.

## Known Gaps

- This document predates the API gateway (`gateway/`), which now exists and is fully implemented. The gateway enforces mandatory Bearer auth (`Authorization: Bearer <GATEWAY_AUTH_TOKEN>`) on every proxied `/v1/*` route and applies in-memory token-bucket rate limiting; `/health` and `/health/{service}` are intentionally left open for infra probes. See `gateway/README.md` for current behavior — don't trust the bullet points below at face value.
- Individual services behind the gateway (`championship-service`, `race-data-service`, etc.) still have no authentication of their own — they rely entirely on the gateway/network topology for access control. Internal ingestion endpoints rely on deployment topology rather than request signing.
- No rate limiting is implemented inside the individual Go services themselves (only at the gateway edge).
- CORS is not configured anywhere in the repo. This is consistent with native mobile/AR clients talking to the gateway, but should be revisited if a browser-based client is ever introduced.

These gaps should be addressed before exposing the services beyond a local or private development network.
