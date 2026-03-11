<!--
##
## OverDrive 2026
## All Technical rights reserved
##
## tree.md - Detailed repository tree and the purpose of each maintained file.
##
-->

# OverDrive Backend - Repository Tree

```text
OD_Backend/
├── cmd/
│   └── api/
│       └── main.go
│          Role:
│          - Backend process entrypoint.
│          - Parses startup flags (`addr`, `year`, `country`, `meeting`, `driver-number`).
│          - Loads runtime config and checks DB connectivity.
│          - Builds the HTTP server through `internal/app`.
│          - Starts the API and handles graceful shutdown.
│
├── internal/
│   ├── api/
│   │   ├── middleware.go
│   │   │  Role:
│   │   │  - HTTP middleware stack.
│   │   │  - Adds request IDs, panic recovery, and structured access logs.
│   │   │
│   │   ├── race_handler.go
│   │   │  Role:
│   │   │  - Main HTTP transport layer.
│   │   │  - Parses query/path params and maps them to service calls.
│   │   │  - Exposes race ingestion, active-race reads, catalog, session-scoped historical reads,
│   │   │    driver resources, standings, metadata, and broadcast endpoints.
│   │   │
│   │   ├── response.go
│   │   │  Role:
│   │   │  - Shared JSON response helpers.
│   │   │  - Centralizes success/error payload writing.
│   │   │
│   │   └── router.go
│   │      Role:
│   │      - Declares all HTTP routes.
│   │      - Registers root, health, catalog, active-race endpoints, and session-scoped endpoints.
│   │      - Applies the middleware chain.
│   │
│   ├── app/
│   │   └── server.go
│   │      Role:
│   │      - Dependency wiring layer.
│   │      - Connects provider client -> use case -> repository -> service -> handler -> router.
│   │      - Builds the final `http.Server`.
│   │
│   ├── config/
│   │   └── config.go
│   │      Role:
│   │      - Central runtime configuration.
│   │      - Reads env vars and applies defaults for API, OpenF1, timeouts, retry policy, and throttling.
│   │
│   ├── database/
│   │   └── database.go
│   │      Role:
│   │      - Prisma DB client bootstrap.
│   │      - Connects, disconnects, and runs basic DB liveness checks.
│   │
│   ├── domain/
│   │   ├── catalog.go
│   │   │  Role:
│   │   │  - Domain types for catalog navigation.
│   │   │  - Defines championship, race, and session summaries exposed by the API.
│   │   │  - Includes session-level broadcast URL metadata.
│   │   │
│   │   └── race_archive.go
│   │      Role:
│   │      - Core backend payload model.
│   │      - Defines `RaceArchive`, metadata, counts, datasets, and archive-oriented response types.
│   │
│   ├── providers/
│   │   └── openf1/
│   │       ├── client.go
│   │       │  Role:
│   │       │  - HTTP adapter for OpenF1.
│   │       │  - Handles URL building, retries, throttling, timeouts, and provider-specific HTTP errors.
│   │       │
│   │       └── endpoints.go
│   │          Role:
│   │          - Defines which OpenF1 endpoints are fetched.
│   │          - Separates full-session ingestion from driver-focused ingestion.
│   │
│   ├── repository/
│   │   ├── race_archive_store.go
│   │   │  Role:
│   │   │  - Repository contract used by the service layer.
│   │   │  - Defines the interface for storing and reading race archives and catalog data.
│   │   │
│   │   └── prisma/
│   │       ├── race_archive_store.go
│   │       │  Role:
│   │       │  - Prisma repository entrypoints.
│   │       │  - Implements `Store`, `GetLatest`, `GetLatestMerged`, and `GetSessionMerged`.
│   │       │  - Orchestrates atomic writes of archive row + raw chunks + normalized tables.
│   │       │
│   │       ├── race_archive_catalog.go
│   │       │  Role:
│   │       │  - Read-side catalog and archive reconstruction logic.
│   │       │  - Loads championships, events, sessions, rebuilds merged archives from stored chunks,
│   │       │    and resolves session driver broadcast URLs.
│   │       │
│   │       ├── race_archive_entities.go
│   │       │  Role:
│   │       │  - Metadata upsert helpers.
│   │       │  - Ensures provider, championship, event, and session rows exist before data writes.
│   │       │
│   │       ├── race_archive_participants.go
│   │       │  Role:
│   │       │  - Raw archive chunk persistence and participant synchronization.
│   │       │  - Splits big datasets into `RaceDatasetChunk` rows.
│   │       │  - Upserts `Team` and `Driver` entities from OpenF1 driver data.
│   │       │  - Maintains the `championship -> team -> driver` participant chain.
│   │       │  - Upserts `SessionDriverBroadcast` rows with one broadcast URL per driver/session.
│   │       │
│   │       ├── race_archive_normalized.go
│   │       │  Role:
│   │       │  - Normalized session storage.
│   │       │  - Clears stale session rows and bulk-inserts laps, telemetry, positions, weather, standings, and other race datasets.
│   │       │
│   │       └── race_archive_helpers.go
│   │          Role:
│   │          - Shared helper functions for the Prisma repository package.
│   │          - Contains JSON helpers, row readers, inference helpers, broadcast URL builders,
│   │          - deduplication helpers, chunking, and UUID generation.
│   │
│   ├── service/
│   │   └── race_service.go
│   │      Role:
│   │      - Application service layer.
│   │      - Coordinates build/store operations and exposes active-race and session-scoped read methods.
│   │      - Serializes concurrent ingestion calls with a mutex.
│   │
│   └── usecase/
│       └── build_race.go
│          Role:
│          - Core race ingestion use case.
│          - Finds the meeting, finds the race session, fetches all configured datasets, applies fallback logic, and builds `domain.RaceArchive`.
│
├── docs/
│   ├── db_schema.mmd
│   │  Role:
│   │  - Mermaid ER diagram for the database schema.
│   │
│   ├── endpoint.md
│   │  Role:
│   │  - API route reference.
│   │  - Documents public endpoints, parameters, and returned payloads.
│   │
│   ├── tree.md
│   │  Role:
│   │  - This file.
│   │  - High-level tree of the repository and the role of each maintained file.
│   │
│   └── workflow.md
│      Role:
│      - Functional walkthrough of the backend.
│      - Explains runtime flow and repository/service behavior function by function.
│
├── resources/
│   ├── database_scheme.sql
│   │  Role:
│   │  - SQL reference snapshot for the database structure.
│   │
│   └── schema.prisma
│      Role:
│      - Source of truth for the Prisma schema.
│      - Declares auth, catalog, archive, normalized race tables, and broadcast storage.
│
├── tests/
│   └── internal/
│       ├── api/
│       │   ├── catalog_router_test.go
│       │   ├── race_handler_test.go
│       │   └── router_test.go
│       │  Role:
│       │  - HTTP-layer tests for handlers, route registration, and middleware behavior.
│       │
│       ├── app/
│       │   └── server_test.go
│       │  Role:
│       │  - Wiring test for `internal/app`.
│       │
│       ├── config/
│       │   └── config_test.go
│       │  Role:
│       │  - Config loading tests.
│       │
│       ├── database/
│       │   └── database_test.go
│       │  Role:
│       │  - DB wrapper tests without a real database.
│       │
│       ├── mocks/
│       │   ├── fixtures.go
│       │   ├── http_transport.go
│       │   └── race_archive_store_mock.go
│       │  Role:
│       │  - Shared test fixtures and mocked dependencies.
│       │  - Replaces the DB repository and outbound HTTP transport in unit tests.
│       │
│       ├── service/
│       │   └── race_service_test.go
│       │  Role:
│       │  - Service-layer tests with repository mocks.
│       │
│       └── usecase/
│           └── build_race_test.go
│          Role:
│          - Use-case tests for OpenF1 ingestion and fallback logic.
│
├── .github/
│   ├── ISSUE_TEMPLATE/
│   │   Role:
│   │   - GitHub issue templates.
│   │
│   ├── PULL_REQUEST_TEMPLATE.md
│   │   Role:
│   │   - Pull request template.
│   │
│   └── workflows/
│       └── mirroring.yml
│          Role:
│          - GitHub Actions workflow for repository mirroring.
│
├── README.md
│   Role:
│   - Project overview, local setup notes, and API usage examples.
│
├── go.mod
│   Role:
│   - Go module definition and dependency list.
│
├── go.sum
│   Role:
│   - Locked Go dependency hashes.
│
├── package.json
│   Role:
│   - Node-side tooling dependencies used locally for Prisma Studio.
│
├── package-lock.json
│   Role:
│   - Locked npm dependency graph for the local Prisma tooling setup.
│
└── .gitignore
    Role:
    - Ignore rules for generated files, local tooling artifacts, and machine-specific files.
```

## Runtime Flow

```text
HTTP request
  -> internal/api/router.go
  -> internal/api/middleware.go
  -> internal/api/race_handler.go
  -> internal/service/race_service.go
  -> internal/usecase/build_race.go
  -> internal/providers/openf1/client.go
  -> OpenF1 API
  -> domain.RaceArchive
  -> internal/repository/prisma/*
  -> PostgreSQL
  -> HTTP JSON response
```

## Read Flow

```text
HTTP request
  -> internal/api/race_handler.go
  -> internal/service/race_service.go
  -> internal/repository/prisma/race_archive_store.go
  -> internal/repository/prisma/race_archive_catalog.go
  -> rebuild or merge archive from PostgreSQL for either:
     - latest stored session
     - explicit sessionId
  -> HTTP JSON response
```

## Main Technical Boundaries

- `internal/api`: HTTP transport only.
- `internal/service`: application orchestration.
- `internal/usecase`: provider-driven ingestion logic.
- `internal/providers`: external API adapters.
- `internal/repository`: persistence contracts and implementations.
- `internal/domain`: shared business payloads.
- `tests/`: unit tests mirrored on the production package structure.
