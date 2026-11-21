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
prefix (`/v1/auth`, `/v1/presets`, `/v1/championship`, `/v1/race-data`,
`/v1/ingestion`, plus `/auth` and `/presets` during the transition) is reachable once the
stack is up. Authentication routes are public; data, presets and sessions need the user's
access token from `POST /v1/auth/login`; ingestion needs an `admin` access token or the
gateway token - see `gateway/ROUTES.md`.

## Default ports

- `gateway`: `3000`
- `auth-service`: `3001`
- `user-data-service`: `3002`
- `championship-service`: `3003`
- `race-data-service`: `3004`
- `ingestion-service`: `3005`

Only the gateway port is published to the network. The service ports and Postgres (`5432`) are
bound to `127.0.0.1`: reachable from your machine for debugging, not from another one.

Each service also has a `.env.template` to run it alone outside Docker (`go run`); it holds no
secret value. Local `.env` files are ignored by git.

## Root environment file

Docker Compose reads the root `.env` (copy it from `.env.template`). Secrets ship empty: generate
each one with `openssl rand -hex 32` (hex, because the Postgres password is embedded in database
URLs). Upgrading a checkout that already has a database: see "Upgrading an existing checkout" in
`docs/getting-started.md`. Required keys:

| Variable | Used by | Notes |
| --- | --- | --- |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `postgres`, every service's database URL | `POSTGRES_PASSWORD` must be set |
| `GATEWAY_AUTH_TOKEN` | `gateway` | **required**, at least 32 bytes - bearer token for internal and scripted calls, accepted on `/v1/ingestion` only; never shipped in a client |
| `GATEWAY_RATE_LIMIT_RPS` / `GATEWAY_RATE_LIMIT_BURST` | `gateway` | rate limiter tuning |
| `GATEWAY_TRUSTED_PROXIES` | `gateway` | optional: IPs/CIDRs of reverse proxies in front of the gateway, whose `X-Forwarded-For` identifies clients for rate limiting; empty without a proxy |
| `AUTH_JWT_SECRET` | `auth-service`, `gateway`, `user-data-service` | **required**, at least 32 bytes and not the template placeholder - the three refuse to start otherwise |
| `INTERNAL_INGESTION_TOKEN` | `ingestion-service`, `championship-service`, `race-data-service` | **required**, at least 32 bytes - protects `POST /internal/ingestion/batches` (sent by the ingestion dispatcher in `X-Internal-Token`) |

## Database setup

The backend uses one PostgreSQL database per service.
`tools/prisma.sh` resolves the service-specific `.env` file automatically based on the selected service.

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

`tools/prisma.sh` runs any Prisma CLI command on one service schema with the CLI embedded in
`prisma-client-go`, the same version the `*-dbsync` jobs use to push the schemas, so it only needs
Go. Studio is the exception: the embedded CLI cannot run it, so the script starts it with
`npx prisma@<embedded version>`, which needs Node.js.

Validate the four schemas (`auth`, `user-data`, `championship`, `race-data`):

```bash
tools/prisma.sh all validate
```

Open Prisma Studio on a running stack:

```bash
tools/prisma.sh championship studio --port 5555
tools/prisma.sh race-data studio --port 5556
```

The script uses the schema's `*_DATABASE_URL` when it is set (environment, then the service
`.env`); otherwise it builds the URL from `POSTGRES_USER` / `POSTGRES_PASSWORD` in the root
`.env` for `localhost:5432`. The `.env` files are parsed, never executed.

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
Because there is no root Go module, every check runs **per module** across the ten
modules (`gateway`, the five `services/*`, `shared/apierror`, `shared/authjwt`,
`shared/bootstrap`, `shared/contracts`), with Go 1.27:

| Check | What it runs |
| --- | --- |
| `lint` | `gofmt -l` must report nothing (excluding generated `resources/db`), then `go vet`, `staticcheck`, `gosec` (generated code excluded) and `govulncheck` on `./...` |
| `test` | `go test ./... -race` |
| `build` | `go build ./...` in the workspace, then again with `GOWORK=off`: each `go.mod` declares its real dependencies and the `replace` directives for `shared/*`, so a module builds on its own |

An accepted analyzer finding carries an inline `//lint:ignore <check> <reason>` (staticcheck) or
`// #nosec <rule> -- <reason>` (gosec). Analyzer versions are pinned in `ci.yml`.

GitHub Actions are pinned by commit SHA. Dependencies, actions and images are updated by hand.

Each check is a matrix of per-module jobs (`lint (gateway)`, `test (gateway)`, ...) plus
an aggregate job with the stable name `lint`, `test`, or `build` - those three are the
names to use as required status checks in branch protection.

The Prisma Go clients under `services/*/resources/db` are generated artifacts and are
gitignored, so CI regenerates them (`go run github.com/steebchen/prisma-client-go
generate`) for `auth-service`, `user-data-service`, `championship-service`, and
`race-data-service` before building or testing them. Generation needs no database connection.

Reproduce a check locally, e.g.:

```bash
cd services/championship-service && gofmt -l . && go vet ./... && go test ./... -race && go build ./... && GOWORK=off go build ./...
```

## Security

See `docs/security.md` for the current local security posture, known gaps, and deployment notes.

## Next steps

- replace placeholder service info use cases with real domain use cases in `user-data-service`
- add service-specific controllers, routes, entities, and repositories for `user-data-service`
