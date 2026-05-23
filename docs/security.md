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

Docker Compose currently uses development credentials:

- PostgreSQL user: `postgres`
- PostgreSQL password: `postgres`

These values are acceptable for local development only. Production should source database credentials from the deployment secret manager.

## Known Gaps

- No authentication or authorization is enforced yet on active endpoints.
- Internal ingestion endpoints rely on deployment topology rather than request signing.
- No rate limiting is implemented in the Go services.
- CORS is not configured because the active services are backend APIs intended to be reached by service clients or a future gateway.

These gaps should be addressed before exposing the services beyond a local or private development network.
