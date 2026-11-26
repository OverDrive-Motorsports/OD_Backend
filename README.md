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

Every binary also accepts one argument, `healthcheck`: it probes its own `/health` on
`HTTP_PORT` and exits non-zero when unhealthy (the container health check of the images, which
have no shell nor curl).

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
secret value. Local `.env` files are ignored by git. They are optional: a binary reads them when
present, then requires its variables from the environment (the images ship no `.env`).

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
| `OPENF1_USERNAME` / `OPENF1_PASSWORD` / `OPENF1_TOKEN_URL` | `ingestion-service` | optional: OpenF1 account, required by OpenF1 for live data (both or neither), and its token endpoint |
| `OPENF1_LIVE_POLL_INTERVAL` / `OPENF1_LIVE_GRACE` / `OPENF1_LIVE_MAX_FAILURES` / `OPENF1_LIVE_MAX_JOBS` | `ingestion-service` | optional: live job tuning (`3s`, `30m`, `20`, `1`) |
| `GOOGLE_OAUTH_CLIENT_ID` / `GOOGLE_OAUTH_CLIENT_SECRET` / `GOOGLE_OAUTH_REDIRECT_URL` | `auth-service` | optional: Google sign-in, disabled when empty |
| `OVERDRIVE_IMAGE_TAG` | Compose | optional: tag of the built images (default `local`) |

## Database setup

The backend uses one PostgreSQL database per service.
`tools/prisma.sh` resolves the service-specific `.env` file automatically based on the selected service.

Start only PostgreSQL:

```bash
docker compose up -d postgres
```

Start the whole stack (schema sync jobs run first, then the services, from their production
images built from the repository):

```bash
docker compose up -d --build
```

Develop from the mounted sources instead (each binary built with `go build` in `golang:1.27`,
no image rebuild; `docker compose restart <service>` after a change):

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d
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
| `test` | `tools/coverage.sh <module>`: `go test ./... -race` with coverage, failing below the module's minimum in `tools/coverage-thresholds.txt`; plus the database integration tests of the four Prisma services against a `postgres:16` service container (`tools/integration-test.sh <service>`) |
| `build` | `go build ./...` in the workspace, then again with `GOWORK=off`: each `go.mod` declares its real dependencies and the `replace` directives for `shared/*`, so a module builds on its own; then the production image of every binary (`docker build -f <module>/Dockerfile .`), checked to run as `nonroot` with no Go source nor `.env` inside |

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

### Coverage gate

`tools/coverage.sh` (no argument: every module) runs the unit tests with the race detector and
prints each module's total coverage. Every package of the module counts (`-coverpkg`, so tests
kept in another package, like `shared/apierror/test`, are measured); the generated
`resources/db` clients are excluded. The job fails when a module is below its minimum in
`tools/coverage-thresholds.txt`. A minimum is the module's coverage rounded down when it was
set: raise it by hand when coverage goes up, and justify any decrease in the pull request.
Totals differ between Go patch releases for the same code, so the script always measures with
the toolchain named on the file's `toolchain` line (the go command downloads it if needed);
bumping it means re-measuring every minimum in the same change.

### Database integration tests

The `TestIntegration*` tests of each Prisma repository package run against a real Postgres:
preset limit and single default under concurrency (user-data), create-only event upsert on
re-ingestion (championship), the replay catalog `EXISTS` query (race-data), refresh token
rotation, session revocation, the OAuth sign-up transaction and concurrent registration
(auth). Without `INTEGRATION_DATABASE_URL` they are skipped, so `go test ./...` needs no
database. Run them locally with one command (Docker required):

```bash
tools/integration-test.sh            # the four services
tools/integration-test.sh user-data  # one service
```

It starts a throwaway `postgres:16.16-alpine` container on a random localhost port, pushes each
schema into its own `overdrive_<service>_test` database, runs the tests and removes the
container; the Compose stack and its data are not touched. It fails when a test is skipped or
none ran. In CI, `INTEGRATION_POSTGRES_URL` points the script at the job's Postgres service
container instead (server only, no database name). The tests refuse a database whose name does
not end with `_test`, checked again on the connection itself (`current_database()`).

## Security

See `docs/security.md` for the current local security posture, known gaps, and deployment notes.

## Next steps

- replace placeholder service info use cases with real domain use cases in `user-data-service`
- add service-specific controllers, routes, entities, and repositories for `user-data-service`
