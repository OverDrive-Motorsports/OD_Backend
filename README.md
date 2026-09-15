# OverDrive Backend

This repository contains a Go monorepo split into a gateway and multiple services.
It is the backend for the OverDrive AR platform and mobile app, which consume it
exclusively through the gateway's versioned `/v1/...` routes.

## Gateway

`gateway/` is a centralized HTTP entrypoint that proxies requests to the internal
services, applying auth, logging, rate limiting, and route-level validation.
See [gateway/README.md](./gateway/README.md) for setup and usage, and
[gateway/ROUTES.md](./gateway/ROUTES.md) for the full route mapping.

## Services

- `auth-service`
- `user-data-service`
- `championship-service`
- `race-data-service`
- `ingestion-service`

## Target structure

Each service now follows the same hexagonal structure, adapted to Go:

```text
service-name/
├── src/
│   ├── core/
│   │   ├── domain/
│   │   ├── ports/
│   │   └── usecases/
│   └── adapters/
│       └── http/
├── doc/
│   └── endpoint.md
├── .env.template
├── .env
├── README.md
├── go.mod
└── main.go
```

## Layer responsibilities

- `src/core/domain`: pure business entities
- `src/core/ports`: contracts exposed by the core
- `src/core/usecases`: application use cases
- `src/adapters/http`: incoming HTTP entrypoints
- `doc/endpoint.md`: endpoint documentation for the service
- `.env.template`: environment template for local setup

## Run a service

From the repository root:

```bash
go run ./services/auth-service
go run ./services/user-data-service
go run ./services/championship-service
go run ./services/race-data-service
go run ./services/ingestion-service
```

Run the gateway:

```bash
go run ./gateway
```

Every service exposes:

- `GET /health`

The active Docker Compose stack runs the full backend:

- `auth-service`
- `user-data-service`
- `championship-service`
- `race-data-service`
- `ingestion-service`
- `gateway`
- PostgreSQL

`gateway` requires all five services to be healthy before starting, so every proxied
prefix (`/auth`, `/presets`, `/providers`, `/users`, `/v1/championship`,
`/v1/race-data`, `/v1/ingestion`) is reachable once the stack is up.
`user-data-service` is still scaffolding at the code level (health check only), but it
is started and health-checked like the rest.

## Default ports

- `gateway`: `3000`
- `auth-service`: `3001`
- `user-data-service`: `3002`
- `championship-service`: `3003`
- `race-data-service`: `3004`
- `ingestion-service`: `3005`

Each service includes a scaffolded `.env.template`. Local `.env` files are ignored by git.

## Root environment file

Docker Compose reads the root `.env` (copy it from `.env.template`). Required keys:

| Variable | Used by | Notes |
| --- | --- | --- |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `postgres`, every service's database URL | dev credentials only |
| `GATEWAY_AUTH_TOKEN` | `gateway` | bearer token required on every `/v1/...` route |
| `GATEWAY_RATE_LIMIT_RPS` / `GATEWAY_RATE_LIMIT_BURST` | `gateway` | rate limiter tuning |
| `AUTH_JWT_SECRET` | `auth-service` | **required** — `auth-service` exits at startup if it is unset (there is no insecure fallback), so the container will crash-loop without it |

## Database setup

The backend uses one PostgreSQL database per service.
Root Prisma commands resolve the service-specific `.env` file automatically based on the selected schema.

Start only PostgreSQL:

```bash
docker compose up -d postgres
```

Start the whole stack (schema sync jobs run first, then the services):

```bash
docker compose up -d
```

Useful endpoints once the stack is up:

- `http://localhost:3001/health`
- `http://localhost:3002/health`
- `http://localhost:3003/health`
- `http://localhost:3004/health`
- `http://localhost:3005/health`
- `http://localhost:3000/health` (gateway, no auth required)

Stop the stack:

```bash
docker compose down
```

| Service | Environment variable | Default local database |
| --- | --- | --- |
| `auth-service` | `AUTH_DATABASE_URL` | `overdrive_auth` |
| `user-data-service` | `USER_DATA_DATABASE_URL` | `overdrive_user_data` |
| `championship-service` | `CHAMPIONSHIP_DATABASE_URL` | `overdrive_championship` |
| `race-data-service` | `RACE_DATA_DATABASE_URL` | `overdrive_race_data` |

This separation is required to keep database ownership aligned with the multi-service architecture.
Do not point multiple services to the same physical database in deployment.

## Prisma utilities

Install the local Node dev dependencies before running Prisma Studio or validation commands:

```bash
npm install
```

Validate the active Prisma schemas:

```bash
npm run prisma:validate:active
```

All four schemas (`auth`, `user-data`, `championship`, `race-data`) are pushed by their
own `*-dbsync` Compose job. `npm run prisma:validate:active` only covers the
`championship` and `race-data` schemas; use `npm run prisma:validate:auth` /
`npm run prisma:validate:user-data` for the other two.

Open Prisma Studio for the active databases:

```bash
CHAMPIONSHIP_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/overdrive_championship?schema=public' \
npx prisma studio --schema services/championship-service/resources/schema.prisma --port 5555

RACE_DATA_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/overdrive_race_data?schema=public' \
npx prisma studio --schema services/race-data-service/resources/schema.prisma --port 5556
```

## Ingestion quick start

For a new OpenF1 Race session, ingest catalog data first:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["meetings","sessions","drivers","session_result"],"dispatch":true}'
```

Then ingest standings and grid data:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["championship_drivers","championship_teams","starting_grid"],"dispatch":true}'
```

Then ingest race-data resources in manageable batches. Large driver-scoped resources such as `car_data` and `location` should include `driver_number` when possible.

## Continuous integration

`.github/workflows/ci.yml` runs on every push (all branches) and on every pull request.
Because there is no root Go module, every check runs **per module** across the nine
modules (`gateway`, the five `services/*`, `shared/apierror`, `shared/bootstrap`,
`shared/contracts`):

| Check | What it runs |
| --- | --- |
| `lint` | `gofmt -l` must report nothing (excluding generated `resources/db`) and `go vet ./...` |
| `test` | `go test ./... -race` |
| `build` | `go build ./...` |

Each check is a matrix of per-module jobs (`lint (gateway)`, `test (gateway)`, ...) plus
an aggregate job with the stable name `lint`, `test`, or `build` — those three are the
names to use as required status checks in branch protection.

The Prisma Go clients under `services/*/resources/db` are generated artifacts and are
gitignored, so CI regenerates them (`go run github.com/steebchen/prisma-client-go
generate`) for `auth-service`, `championship-service`, and `race-data-service` before
building or testing them. Generation needs no database connection.

Reproduce a check locally, e.g.:

```bash
cd services/championship-service && gofmt -l . && go vet ./... && go test ./... -race && go build ./...
```

## Security

See `docs/security.md` for the current local security posture, known gaps, and deployment notes.

## Next steps

- replace placeholder service info use cases with real domain use cases in `user-data-service`
- add service-specific controllers, routes, entities, and repositories for `user-data-service`
