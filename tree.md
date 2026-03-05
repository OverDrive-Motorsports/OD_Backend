<!--
##
## OverDrive 2026
## All Technical rights reserved
##
## tree.md - Detailed architecture tree and responsibilities for each module.
##
-->

# OverDrive Backend - Architecture Tree (V1)

```text
OD_Backend/
├── cmd/
│   └── api/
│       └── main.go
│          Role:
│          - Application entrypoint for the backend.
│          - Reads config/flags (`addr`, `year`, `country`, `meeting`, `driver-number`).
│          - Initializes the JSON logger.
│          - Builds the HTTP server through `internal/app`.
│          - Handles lifecycle (listen + graceful shutdown on SIGINT/SIGTERM).
│
├── internal/
│   ├── app/
│   │   └── server.go
│   │      Role:/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## [FileName] - [Brief description of the file's purpose]
 ## Example: "HomePage - Main component for the showcase website's homepage."
 ##
 */
│   │      - Runtime dependency wiring (provider -> usecase -> service -> cache -> handler -> router).
│   │      - Builds `http.Server` with server timeouts.
│   │      - Single place to plug in new adapters (DB/Redis/etc.).
│   │
│   ├── api/
│   │   ├── router.go
│   │   │  Role:
│   │   │  - Declares all routes.
│   │   │  - Exposes V1 routes:
│   │   │    - `GET /api/v1/race/getrace`
│   │   │    - `POST /api/v1/race/sendrace`
│   │   │    - `GET /api/v1/race/cache`
│   │   │  - Keeps legacy aliases:
│   │   │    - `GET /getrace`
│   │   │    - `POST /sendrace`
│   │   │  - Applies middleware chain (request-id, recovery, access logs).
│   │   │
│   │   ├── middleware.go
│   │   │  Role:
│   │   │  - `withRequestID`: injects/propagates `X-Request-ID`.
│   │   │  - `withRecovery`: protects against panics and returns JSON 500.
│   │   │  - `withAccessLog`: writes structured logs for each HTTP request.
│   │   │  - `chain`: composes middleware stack.
│   │   │
│   │   ├── response.go
│   │   │  Role:
│   │   │  - JSON response helpers (`writeJSON`, `writeError`).
│   │   │  - Standard error format `{"error": "..."}`.
│   │   │
│   │   └── race_handler.go
│   │      Role:
│   │      - HTTP transport layer (query parsing/validation + status mapping).
│   │      - Endpoints:
│   │        - `HandleRoot`: service info/routes.
│   │        - `HandleHealth`: healthcheck.
│   │        - `HandleGetRace`: fetches OpenF1 data and caches it in memory.
│   │        - `HandleSendRace`: returns latest cached race payload.
│   │        - `HandleCacheStatus`: cache state endpoint.
│   │      - Converts query params into `usecase.RaceBuildInput`.
│   │
│   ├── config/
│   │   └── config.go
│   │      Role:
│   │      - Centralized config (env vars + defaults):
│   │        - OpenF1 (`OPENF1_BASE_URL`, timeout/retry/rate limit)
│   │        - API (`API_ADDR`, `GETRACE_TIMEOUT`, `API_SHUTDOWN_TIMEOUT`)
│   │      - String/int/duration parsing helpers.
│   │
│   ├── domain/
│   │   └── race_archive.go
│   │      Role:
│   │      - Shared domain models.
│   │      - `RaceArchive`: normalized payload returned by `sendrace`.
│   │      - `Metadata`: extraction context (meeting/session/driver/etc.).
│   │
│   ├── providers/
│   │   └── openf1/
│   │       ├── endpoints.go
│   │       │  Role:
│   │       │  - Defines OpenF1 endpoint lists to collect.
│   │       │  - Two modes:
│   │       │    - `RaceSessionEndpoints` (full race session)
│   │       │    - `DriverFocusedEndpoints` (single-driver mode)
│   │       │
│   │       └── client.go
│   │          Role:
│   │          - OpenF1 HTTP client.
│   │          - Handles retries, throttling, timeouts, typed HTTP errors.
│   │          - Builds URLs and decodes JSON array payloads.
│   │
│   ├── usecase/
│   │   └── build_race.go
│   │      Role:
│   │      - Core business flow for "build race archive".
│   │      - Business pipeline:
│   │        1) find meeting (`/meetings`)
│   │        2) find race session (`/sessions`)
│   │        3) fetch datasets endpoint by endpoint
│   │        4) apply smart fallbacks (e.g. `422 too much data`, `404 no results`)
│   │        5) build `domain.RaceArchive` + counts + partial fetch errors
│   │      - Supports `driver_number` filter mode.
│   │
│   ├── service/
│   │   └── race_service.go
│   │      Role:
│   │      - Application orchestration between usecase and repository.
│   │      - `FetchAndCache`: runs usecase and stores payload in cache.
│   │      - `GetCached`: reads latest snapshot.
│   │      - Uses mutex to prevent duplicate burst fetches.
│   │
│   └── repository/
│       ├── race_cache.go
│       │  Role:
│       │  - Cache persistence interface.
│       │  - Stable contract for future DB/Redis implementations.
│       │
│       └── memory/
│           └── race_cache.go
│              Role:
│              - In-memory thread-safe cache implementation.
│              - V1 without database.
│
├── .github/
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug.yml
│   │   ├── config.yml
│   │   ├── enhancement.yml
│   │   ├── feature.yml
│   │   └── general.yml
│   │      Role:
│   │      - GitHub issue creation templates.
│   │
│   ├── PULL_REQUEST_TEMPLATE.md
│   │  Role:
│   │  - Standard pull request template.
│   │
│   └── workflows/
│       └── mirroring.yml
│          Role:
│          - GitHub Actions workflow for repository mirroring.
│
├── README.md
│   Role:
│   - Project overview + backend run guide + endpoint overview.
│
├── go.mod
│   Role:
│   - Go module declaration (`overdrive`) and Go version.
│
└── .gitignore
    Role:
    - Git ignore rules for generated/local artifacts.
```

## Runtime Flow (Simplified)

```text
HTTP Request
  -> internal/api/router + middlewares
  -> internal/api/race_handler
  -> internal/service/race_service
  -> internal/usecase/build_race
  -> internal/providers/openf1/client
  -> OpenF1 API
  -> returns RaceArchive
  -> internal/repository/memory/race_cache
  -> HTTP JSON Response
```

## Planned Extension Points (when DB is added)

- Replace `internal/repository/memory/race_cache.go` with a Postgres/Redis adapter implementing `RaceCacheRepository`.
- Keep `race_handler` and `race_service` mostly unchanged thanks to repository interface abstraction.
- Add `internal/repository/postgres` or `internal/repository/redis` packages without breaking current API behavior.
