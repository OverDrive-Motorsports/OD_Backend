# OverDrive API Gateway

Centralized HTTP gateway for the OverDrive backend microservices.

## Goals

- Expose a single entrypoint for backend APIs
- Route requests to internal services
- Apply shared cross-cutting concerns (auth, logging, rate limiting)
- Expose health endpoints for gateway and each service

## Run

From repository root:

```bash
go run ./gateway
```

Gateway configuration is loaded from `gateway/.env`.
No runtime fallback values are hardcoded in the gateway config.

## Routes

- Full route mapping: [ROUTES.md](./ROUTES.md)

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

- Optional Bearer auth on all routes (`Authorization: Bearer <GATEWAY_AUTH_TOKEN>`)
- Request logging (method, path, status, duration, remote)
- In-memory token-bucket rate limiting by client IP

If no `Authorization` header is provided, request is allowed.
If an `Authorization` header is provided, it must match the configured token.

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
