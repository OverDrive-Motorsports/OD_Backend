# OverDrive API Gateway

Centralized HTTP gateway for the OverDrive backend microservices.

## Goals

- Expose a single entrypoint for backend APIs
- Route requests to internal services
- Apply shared cross-cutting concerns (auth, logging, rate limiting)
- Expose health endpoints for gateway and each service

## Start Gateway

Prerequisites:

- Go installed (same version as project toolchain)
- `gateway/.env` configured (or environment variables exported)
- Upstream services running on configured URLs

From repository root:

```bash
go run ./gateway
```

From `gateway/` directory:

```bash
go run .
```

Gateway starts on `http://localhost:${HTTP_PORT}` (default template: `3000`).

## Use Gateway

Read configured token from `.env`:

```bash
GATEWAY_TOKEN="$(grep '^GATEWAY_AUTH_TOKEN=' gateway/.env | cut -d= -f2-)"
```

Liveness (no auth required, kept open for infra probes):

```bash
curl -i http://localhost:3000/health
```

Invalid token example on a protected `/v1/*` route:

```bash
curl -i -H "Authorization: Bearer invalid" http://localhost:3000/v1/championship/championships
```

Missing header example (now rejected — see Middleware section below):

```bash
curl -i http://localhost:3000/v1/championship/championships
```

Proxied API examples:

```bash
curl -i -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
  http://localhost:3000/v1/championship/championships

curl -i -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
  http://localhost:3000/v1/race-data/championships
```

Dataset validation examples:

```bash
curl -i -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
  http://localhost:3000/v1/championship/sessions/123/datasets/not_allowed

curl -i -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
  http://localhost:3000/v1/race-data/sessions/123/drivers/not_a_number/profile
```

## Routes

- Full route mapping: [ROUTES.md](./ROUTES.md)
- Championship public facade: `/v1/championship/*` -> `championship-service`
- Race public facade: `/v1/race-data/*` -> `race-data-service`

## Hexagonal Architecture (simple)

The gateway uses a lightweight but strict hexagonal separation:

- `internal/core`: business/application logic
  - `domain`: domain models
  - `ports`: contracts/interfaces
  - `usecases`: orchestration and rules
- `internal/adapters`: technical implementations
  - `inbound/http`: HTTP handlers, middleware, reverse proxy
  - `outbound`: auth implementations
- `internal/app`: composition root for dependency wiring
- `main.go`: process bootstrap only

Dependency direction:

- `core` does not depend on `adapters`
- `adapters` depend on `core` ports/use cases
- `app` wires concrete adapters into use cases

## Middleware

Applied at gateway level:

- Mandatory Bearer auth on all `/v1/*` proxied routes (`Authorization: Bearer <GATEWAY_AUTH_TOKEN>`); `/health` and `/health/{service}` stay open for infra probes
- Request logging (method, path, status, duration, remote)
- In-memory token-bucket rate limiting by client IP

Additional route-specific validation:

- `/v1/championship/sessions/{sessionId}/datasets/{dataset}` validates dataset enum at gateway edge
- `/v1/championship/drivers/{driverNumber}/profile` validates `driverNumber` is a positive integer
- `/v1/championship/*?driverNumber=` validates the `driverNumber` query parameter is a positive integer when present
- `/v1/race-data/sessions/{sessionId}/datasets/{dataset}` validates race dataset enum at gateway edge
- `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/{segment}` validates `segment` (driver dataset or `profile`/`broadcast`) at gateway edge
- `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/...` validates `driverNumber` is a positive integer
- `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/laps/{lapNumber}/location` validates `lapNumber` is a positive integer
- `/v1/race-data/sessions/{sessionId}/race/{action}` validates `action` against the allowed race-live sub-resources (`position`, `laps`, `stints`, `pitstops`, `control`, `weather`, `radio`); `control` is a `POST` (long-poll) route, all others are `GET`
- `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/{action}` validates `action` against the allowed telemetry sub-resources (`speed`, `engine`, `location`, `intervals`)
- `/v1/race-data/*?driverNumber=` and `?lapNumber=` validate those query parameters are positive integers when present

`/v1/*` proxied routes now require a valid `Authorization: Bearer <token>` header.
A missing header or a header that does not match the configured token both result
in `401 Unauthorized`. (Previously, requests with no `Authorization` header at all
were let through unauthenticated — this was a known bypass and has been fixed.)

## Forwarded Headers

Gateway forwards and enriches:

- `X-Forwarded-Host`
- `X-Forwarded-Proto`
- `X-Forwarded-For`

## Health Endpoints

Gateway liveness:

```http
GET /health
```

Service health (gateway probes upstream `GET /health`):

```http
GET /health/auth
GET /health/user-data
GET /health/championship
GET /health/race-data
GET /health/ingestion
```

Success response format:

```json
{
  "status": "ok",
  "service": "auth"
}
```

Gateway liveness response:

```json
{
  "status": "ok"
}
```

## Configuration

All variables below are required. Set them in `gateway/.env` (or as process env vars).

| Variable | Required | Example | Description |
| --- | --- | --- | --- |
| `HTTP_PORT` | Yes | `3000` | Gateway listen port |
| `GATEWAY_AUTH_TOKEN` | Yes | `admin` | Bearer token for optional auth validation |
| `GATEWAY_RATE_LIMIT_RPS` | Yes | `10` | Rate-limiter refill (tokens/sec) |
| `GATEWAY_RATE_LIMIT_BURST` | Yes | `20` | Rate-limiter burst capacity |
| `AUTH_SERVICE_URLS` | Yes | `http://localhost:3001` | Auth upstream targets |
| `USER_DATA_SERVICE_URLS` | Yes | `http://localhost:3002` | User-data upstream targets |
| `CHAMPIONSHIP_SERVICE_URLS` | Yes | `http://localhost:3003` | Championship upstream targets |
| `RACE_DATA_SERVICE_URLS` | Yes | `http://localhost:3004` | Race-data upstream targets |
| `INGESTION_SERVICE_URLS` | Yes | `http://localhost:3005` | Ingestion upstream targets |

Multi-target example:

```env
AUTH_SERVICE_URLS=http://localhost:3001,http://localhost:3011
```

Load behavior:

- from repo root (`go run ./gateway`): loads `gateway/.env`
- from `gateway/` directory (`go run .`): loads `.env`
- already exported environment variables keep priority over `.env` values

## Project Structure

```text
gateway/
├── main.go
├── README.md
├── ROUTES.md
├── HOWTOCONTRIBUTE.md
├── .env
├── .env.template
└── internal/
    ├── app/
    ├── core/
    │   ├── domain/
    │   ├── ports/
    │   └── usecases/
    └── adapters/
        ├── inbound/
        │   └── http/
        └── outbound/
```

## Contributing

See [HOWTOCONTRIBUTE.md](./HOWTOCONTRIBUTE.md).
