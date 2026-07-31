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

The active Docker Compose stack currently runs:

- `championship-service`
- `race-data-service`
- `ingestion-service`
- `gateway`
- PostgreSQL

`auth-service` and `user-data-service` are scaffolded for later work and are not started by Docker Compose yet.
`gateway` requires `championship-service`, `race-data-service`, and `ingestion-service` to be healthy before starting.
Its proxied routes to `auth-service`/`user-data-service` will return `502` until those services are wired into Compose.

## Default ports

- `gateway`: `3000`
- `auth-service`: `3001`
- `user-data-service`: `3002`
- `championship-service`: `3003`
- `race-data-service`: `3004`
- `ingestion-service`: `3005`

Each service includes a scaffolded `.env.template`. Local `.env` files are ignored by git.

## Database setup

The backend uses one PostgreSQL database per service.
Root Prisma commands resolve the service-specific `.env` file automatically based on the selected schema.

Start only PostgreSQL:

```bash
docker compose up -d postgres
```

Start the microservice stack with schema sync:

```bash
docker compose up -d championship-dbsync race-data-dbsync championship-service race-data-service ingestion-service gateway
```

Useful endpoints once the stack is up:

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

`auth-service` and `user-data-service` schemas are scaffolded and are not part of the active stack yet.

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

## Security

See `docs/security.md` for the current local security posture, known gaps, and deployment notes.

## Next steps

- replace placeholder service info use cases with real domain use cases in `auth-service` and `user-data-service`
- add service-specific controllers, routes, entities, and repositories for `auth-service` and `user-data-service`
- wire `gateway`, `auth-service`, and `user-data-service` into Docker Compose once they carry real logic
