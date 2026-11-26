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
- `gateway/.env` configured, or the variables exported (the file is optional)
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

Get a user access token from the public login route (15 minutes), and keep the gateway token
for the admin routes:

```bash
TOKEN="$(curl -s -X POST http://localhost:3000/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","password":"..."}' | jq -r .token)"
GATEWAY_TOKEN="$(grep '^GATEWAY_AUTH_TOKEN=' gateway/.env | cut -d= -f2-)"
```

Liveness (no auth required, kept open for infra probes):

```bash
curl -i http://localhost:3000/health
```

Invalid token example on a user route (`401 INVALID_TOKEN`, the gateway token gets the same answer):

```bash
curl -i -H "Authorization: Bearer invalid" http://localhost:3000/v1/championship/championships
```

Missing header example (`401 UNAUTHORIZED`):

```bash
curl -i http://localhost:3000/v1/championship/championships
```

Proxied API examples:

```bash
curl -i -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:3000/v1/championship/championships

curl -i -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:3000/v1/race-data/championships
```

Dataset validation examples:

```bash
curl -i -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:3000/v1/championship/sessions/123/datasets/not_allowed

curl -i -H "Authorization: Bearer ${TOKEN}" \
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

- Per-route access level (`domain.RouteAccess`, declared in `internal/config/routes_*.go`): public (`/v1/auth` login, register, refresh, OAuth), user access token (data, presets, sessions) or admin (`/v1/ingestion`: `admin` access token or `GATEWAY_AUTH_TOKEN`); `/health` and `/health/{service}` stay open for infra probes. See `ROUTES.md`
- Request logging (method, path, status, duration, remote)
- In-memory token-bucket rate limiting by client IP

Additional route-specific validation:

- `/v1/championship/sessions/{sessionId}/datasets/{dataset}` validates dataset enum at gateway edge
- `/v1/championship/drivers/{driverNumber}/profile` validates `driverNumber` is a positive integer
- `/v1/championship/*?driverNumber=` validates the `driverNumber` query parameter is a positive integer when present
- `/v1/race-data/sessions/{sessionId}/datasets/{dataset}` validates race dataset enum at gateway edge
- `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/{segment}` validates `segment` (driver dataset or `profile`) at gateway edge
- `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/...` validates `driverNumber` is a positive integer
- `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/laps/{lapNumber}/location` validates `lapNumber` is a positive integer
- `/v1/race-data/sessions/{sessionId}/race/{action}` validates `action` against the allowed race-live sub-resources (`position`, `laps`, `stints`, `pitstops`, `control`, `weather`, `radio`); `control` is a `POST` (long-poll) route, all others are `GET`
- `/v1/race-data/sessions/{sessionId}/drivers/{driverNumber}/telemetry/{action}` validates `action` against the allowed telemetry sub-resources (`speed`, `engine`, `location`, `intervals`)
- `/v1/race-data/*?driverNumber=` and `?lapNumber=` validate those query parameters are positive integers when present

Every proxied route except the public auth routes requires a valid
`Authorization: Bearer <token>` header: `401` (`UNAUTHORIZED`, `TOKEN_EXPIRED` or
`INVALID_TOKEN`) otherwise, and `403 FORBIDDEN` for a `user` token on an admin route.

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

Set them in `gateway/.env` (or as process env vars). Secrets have no default: generate them
with `openssl rand -hex 32`.

| Variable | Required | Example | Description |
| --- | --- | --- | --- |
| `HTTP_PORT` | Yes | `3000` | Gateway listen port |
| `GATEWAY_AUTH_TOKEN` | Yes | *(generated, ≥ 32 bytes)* | Bearer token for internal and scripted calls, accepted on the admin routes only, compared in constant time; never shipped in a client; the gateway refuses to start with a shorter value |
| `AUTH_JWT_SECRET` | Yes | *(generated, ≥ 32 bytes)* | Verifies user access tokens (and their role) on the user and admin routes; the gateway refuses to start with a weak value |
| `GATEWAY_RATE_LIMIT_RPS` | Yes | `10` | Rate-limiter refill (tokens/sec) |
| `GATEWAY_RATE_LIMIT_BURST` | Yes | `20` | Rate-limiter burst capacity |
| `GATEWAY_TRUSTED_PROXIES` | No | `10.0.0.0/8, 192.0.2.7` | Reverse proxies in front of the gateway. Clients are rate-limited by TCP peer address; only these proxies' `X-Forwarded-For` is read (right to left). Empty: no proxy |
| `AUTH_SERVICE_URLS` | Yes | `http://localhost:3001` | Auth upstream targets |
| `USER_DATA_SERVICE_URLS` | Yes | `http://localhost:3002` | User-data upstream targets |
| `CHAMPIONSHIP_SERVICE_URLS` | Yes | `http://localhost:3003` | Championship upstream targets |
| `RACE_DATA_SERVICE_URLS` | Yes | `http://localhost:3004` | Race-data upstream targets |
| `INGESTION_SERVICE_URLS` | Yes | `http://localhost:3005` | Ingestion upstream targets |

Run a single `ingestion-service` instance: its live jobs and their session guard live in memory.

Multi-target example:

```env
AUTH_SERVICE_URLS=http://localhost:3001,http://localhost:3011
```

Load behavior:

- from repo root (`go run ./gateway`): loads `gateway/.env`
- from `gateway/` directory (`go run .`): loads `.env`
- the file is optional: without it (e.g. the production image) every variable comes from the
  environment, and a missing required one stops the start with its name
- `gateway healthcheck` (the container health probe) reads `HTTP_PORT` from the environment or
  the root `.env`, not from `gateway/.env`
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
