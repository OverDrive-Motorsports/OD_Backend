# OverDrive Backend

This repository contains a Go monorepo split into multiple services.
The gateway is intentionally not included yet.

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

Each service currently exposes:

- `GET /health`

## Default ports

- `auth-service`: `3001`
- `user-data-service`: `3002`
- `championship-service`: `3003`
- `race-data-service`: `3004`
- `ingestion-service`: `3005`

Each service includes a scaffolded `.env` file and `.env.template` with `HTTP_PORT` and `APP_VERSION`.

## Next steps

- replace placeholder service info use cases with real domain use cases
- add service-specific controllers, routes, entities, and repositories
- introduce inter-service communication once the gateway is added
