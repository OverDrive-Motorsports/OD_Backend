# OverDrive Backend Workflow

This document explains how the backend works from startup to data delivery, and what each maintained function is doing.

Scope:
- included: `cmd/`, `internal/`, `scripts/`, Prisma repository logic
- excluded: generated ORM code in `resources/db`
- excluded: tests
- note: local bootstrap scripts can differ from the Git-tracked scripts if you keep some of them ignored on your machine

## Global Flow

### 1. Startup flow

1. Developer starts the backend through a local script or directly with `go run ./cmd/api`
2. PostgreSQL must already be available and `DATABASE_URL` must be set
3. [`main.go`](/home/bastou/delivery/eip/OD_Backend/cmd/api/main.go) loads config, connects DB, builds server, starts HTTP
4. The default `driver-number` CLI flag is `0`, so ingestion is full-race by default unless one driver is explicitly requested

### 2. Write flow: fetch and store a race

1. Client calls `GET /api/v1/race/getrace`
2. [`RaceHandler.HandleGetRace`](/home/bastou/delivery/eip/OD_Backend/internal/api/race_handler.go:284) parses query params
3. [`RaceService.FetchAndStore`](/home/bastou/delivery/eip/OD_Backend/internal/service/race_service.go:38) serializes concurrent fetches
4. [`RaceBuilder.Build`](/home/bastou/delivery/eip/OD_Backend/internal/usecase/build_race.go:44) calls OpenF1 and builds a `domain.RaceArchive`
5. [`RaceArchiveStore.Store`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_store.go:50) persists:
   - provider/championship/event/session metadata
   - chunked raw archive datasets in `RaceDatasetChunk`
   - normalized race tables in the same atomic write flow

### 3. Read flow: send data to AR/mobile/web clients

1. Client calls either:
   - `/api/v1/race/sendrace`
   - `/api/v1/race/standings/race`
   - `/api/v1/sessions/{sessionId}/archive`
   - `/api/v1/sessions/{sessionId}/drivers`
   - `/api/v1/sessions/{sessionId}/drivers/{driverNumber}/broadcast`
2. `RaceHandler` reads stored data through `RaceService`
3. `RaceService` calls repository read methods
4. Prisma repository either:
   - rebuilds the merged latest session archive
   - rebuilds the merged archive for one explicit `sessionId`
   - serves catalog and broadcast lookup data
5. Handler serializes JSON response

## Runtime Entry

### `cmd/api/main.go`

| Function | Purpose | Why it exists |
|---|---|---|
| `main` | Bootstraps the whole backend process. | Central entrypoint: loads config, connects DB, builds HTTP server, prints routes, handles graceful shutdown. |

### `internal/app/server.go`

| Function | Purpose | Why it exists |
|---|---|---|
| `NewHTTPServer` | Wires provider client, use case, repository, service, handler, router, and returns `*http.Server`. | Keeps dependency injection in one place instead of spreading wiring across `main`. |

## Config and Database

### `internal/config/config.go`

| Function | Purpose | Why it exists |
|---|---|---|
| `Load` | Builds runtime config from environment variables with defaults. | Keeps the app 12-factor and avoids hardcoded runtime values. |
| `getEnv` | Reads a string env var with fallback. | Small helper for simple string config. |
| `getEnvInt` | Reads an int env var with fallback. | Prevents repeated parse boilerplate. |
| `getEnvDuration` | Reads a duration env var with fallback. | Used for timeouts, retry delays, and polling intervals. |

### `internal/database/database.go`

| Function | Purpose | Why it exists |
|---|---|---|
| `Connect` | Creates and opens the Prisma DB client. | Initializes PostgreSQL access before serving requests. |
| `Disconnect` | Closes the Prisma DB client cleanly. | Used during graceful shutdown. |
| `IsConnected` | Checks that the DB client exists and can execute `SELECT 1`. | Gives a quick liveness check for startup/shutdown logic. |

## OpenF1 Provider

### `internal/providers/openf1/client.go`

| Function | Purpose | Why it exists |
|---|---|---|
| `(*HTTPError).Error` | Formats provider HTTP errors. | Lets upper layers detect `404`, `422`, and other provider-specific situations from one error string. |
| `NewClient` | Creates the OpenF1 HTTP client with timeout, throttling, and retry config. | Centralizes outbound provider client creation. |
| `(*Client).Get` | Calls one OpenF1 endpoint and decodes its JSON array payload. | Main provider access point used by the use case layer. |
| `(*Client).buildURL` | Builds endpoint URLs with query parameters. | Avoids manual URL string concatenation bugs. |
| `(*Client).waitForGap` | Enforces a delay between provider requests. | Prevents burst calls and helps respect provider limits. |
| `sleepWithContext` | Sleeps while honoring context cancellation. | Used during retries without blocking shutdown forever. |
| `trimBody` | Truncates large HTTP error bodies. | Keeps logs and wrapped errors readable. |

## Core Use Case

### `internal/usecase/build_race.go`

This file is the fetch-and-assemble engine.

| Function | Purpose | Why it exists |
|---|---|---|
| `NewRaceBuilder` | Creates the race-building use case. | Injects the OpenF1 client into the use case layer. |
| `(*RaceBuilder).Build` | Fetches meetings, sessions, drivers, and all requested datasets, then builds `domain.RaceArchive`. | This is the main orchestration function for race ingestion. |
| `(*RaceBuilder).fetchDataset` | Fetches one dataset with fallback logic when needed. | Encapsulates OpenF1 quirks like driver-scoped fallback and `404` empty datasets. |
| `(*RaceBuilder).fetchDatasetByDriver` | Re-fetches a dataset once per driver and merges rows. | Used for heavy datasets like `car_data` and `location` when session-wide queries fail with `422`. |
| `shouldFallbackByDriver` | Decides whether an endpoint should retry per driver. | Keeps fallback policy explicit and centralized. |
| `isTooMuchDataError` | Detects OpenF1 `422 too much data`. | Needed to trigger per-driver recovery. |
| `isNoResultsError` | Detects OpenF1 `404 no results found`. | Lets the builder store empty arrays instead of failing the whole archive. |
| `isBadRequestError` | Detects invalid provider requests. | Stops retry strategies when the request itself is invalid. |
| `supportsDriverNumber` | Declares which endpoints accept `driver_number`. | Prevents unsupported query combinations. |
| `extractDriverNumbers` | Extracts a unique driver list from the drivers dataset. | Used to drive per-driver fallback queries. |
| `filterDrivers` | Keeps only one driver in driver-focused mode. | Supports lightweight single-driver archives. |
| `pickMeeting` | Chooses the meeting that best matches the requested name. | Turns raw meeting list into one target event. |
| `pickRaceSession` | Chooses the race session from all sessions of a meeting. | Ensures downstream datasets use the actual race session key. |
| `extractStringOrDefault` | Reads a string field with fallback. | Normalizes loose provider payloads. |
| `extractInt` | Reads an integer field from a provider row. | Used for `meeting_key`, `session_key`, and similar fields. |

## Application Service

### `internal/service/race_service.go`

This layer is thin on purpose. It orchestrates use case + repository calls.

| Function | Purpose | Why it exists |
|---|---|---|
| `NewRaceService` | Creates the application service. | Dependency boundary between handlers and lower layers. |
| `(*RaceService).FetchAndStore` | Builds an archive and persists it. | Main write workflow used by `GET /getrace`. |
| `(*RaceService).GetLatestStored` | Returns the latest raw stored archive. | Used for storage status and raw archive views. |
| `(*RaceService).GetLatestMergedStored` | Returns a merged archive for the latest session. | Important when several driver-focused imports exist for the same event session. |
| `(*RaceService).GetSessionMergedStored` | Returns a merged archive for one explicit session. | Powers historical navigation without relying on the latest import. |
| `(*RaceService).GetSessionDriverBroadcast` | Returns the broadcast URL for one driver in one session. | Powers explicit session-scoped driver broadcast endpoints. |
| `(*RaceService).ListChampionships` | Lists stored championships. | Powers championship catalog endpoints. |
| `(*RaceService).GetChampionshipEvents` | Returns events of one championship. | Powers `championship -> event` navigation. |
| `(*RaceService).GetEvent` | Returns one event summary. | Powers event catalog lookup. |
| `(*RaceService).ListEventSessions` | Returns sessions of one event. | Powers `event -> session` navigation. |
| `(*RaceService).GetSession` | Returns one session summary. | Powers session catalog lookup. |

## HTTP Router and Middleware

### `internal/api/router.go`

| Function | Purpose | Why it exists |
|---|---|---|
| `NewRouter` | Registers all legacy and V1 routes and wraps them with middleware. | Single place where public HTTP surface is defined. |

### `internal/api/middleware.go`

| Function | Purpose | Why it exists |
|---|---|---|
| `chain` | Applies middleware in reverse order around a handler. | Keeps router setup readable. |
| `withRequestID` | Adds or propagates `X-Request-ID`. | Makes logs and client troubleshooting easier. |
| `withRecovery` | Converts panics into `500` JSON responses. | Protects the process from crashing on handler panics. |
| `withAccessLog` | Logs one structured event per request. | Gives basic observability without extra infra. |
| `requestIDFromContext` | Reads the request ID from context. | Shared utility for log entries and error reporting. |
| `(*statusRecorder).WriteHeader` | Captures the response status code. | Needed by access logging middleware. |

### `internal/api/response.go`

| Function | Purpose | Why it exists |
|---|---|---|
| `writeJSON` | Writes JSON responses with status code. | Standard output helper for all handlers. |
| `writeError` | Writes standardized error JSON. | Keeps error payloads consistent. |

## HTTP Handlers

### `internal/api/race_handler.go`

This file is the API surface used by clients.

#### Constructors and generic endpoints

| Function | Purpose | Why it exists |
|---|---|---|
| `NewRaceHandler` | Creates the HTTP handler bundle. | Injects service, defaults, and fetch timeout. |
| `HandleRoot` | Returns service metadata and route list. | Lightweight self-documentation endpoint. |
| `HandleHealth` | Returns liveness info. | Used for health checks and quick manual validation. |
| `HandleGetRace` | Fetches session data from OpenF1 and stores it. | Main ingestion endpoint. |
| `HandleSendRace` | Returns the latest merged full archive. | Main merged latest-session payload endpoint. |
| `HandleStorageStatus` | Tells whether something is stored. | Useful for frontend/backend bootstrap checks. |

#### Catalog endpoints

| Function | Purpose | Why it exists |
|---|---|---|
| `HandleListChampionships` | Lists championships. | Entry point for catalog navigation. |
| `HandleListChampionshipEvents` | Lists events of one championship. | Lets clients browse events by championship. |
| `HandleGetEventCatalog` | Returns one event summary. | Detail view for one event. |
| `HandleListEventSessions` | Lists sessions of one event. | Needed because one event owns practice, qualifying, sprint, and race sessions. |
| `HandleGetSessionCatalog` | Returns one session summary. | Detail view for one session. |

#### Session-scoped read endpoints

| Function | Purpose | Why it exists |
|---|---|---|
| `HandleSendSessionArchive` | Returns the merged archive for one explicit session. | Removes ambiguity when several events are stored in DB. |
| `HandleSendSessionMetadata` | Returns metadata for one explicit session. | Lets clients reconstruct context for historical sessions. |
| `HandleListSessionDatasets` | Lists datasets for one explicit session. | Helps clients discover what is available before fetching. |
| `HandleSendSessionDataset` | Returns one dataset for one explicit session. | Generic session-scoped dataset endpoint. |
| `HandleListSessionDrivers` | Returns drivers for one explicit session. | Lets clients navigate historical event rosters. |
| `HandleListSessionTeams` | Returns teams for one explicit session. | Supports team-centric session views. |
| `HandleSendSessionDriverRace` | Returns the full per-driver payload for one explicit session. | Historical equivalent of the active-session driver payload. |
| `HandleSendSessionDriverProfile` | Returns one driver profile for one explicit session. | Lightweight historical driver lookup. |
| `HandleSendSessionDriverDataset` | Returns one driver-scoped dataset for one explicit session. | Fine-grained session navigation for replay or AR views. |
| `HandleSendSessionWeather` | Returns weather for one explicit session. | Session-scoped weather access. |
| `HandleSendSessionFacts` | Returns race control events for one explicit session. | Session-scoped incidents and flags access. |
| `HandleSendSessionRaceStandings` | Returns standings snapshots for one explicit session. | Replay and history-safe standings lookup. |
| `HandleSendSessionBroadcast` | Returns the session broadcast URL. | Exposes the stored session-wide broadcast link. |
| `HandleSendSessionDriverBroadcast` | Returns the driver broadcast URL for one explicit session. | Exposes the stored driver-specific broadcast link. |

#### Race data endpoints

| Function | Purpose | Why it exists |
|---|---|---|
| `HandleSendDriverChampionship` | Returns driver championship standings dataset. | Used for championship overlays. |
| `HandleSendConstructorChampionship` | Returns constructor standings dataset. | Used for team championship overlays. |
| `HandleSendWeather` | Returns the latest-session weather dataset. | Used for atmosphere and strategy UI. |
| `HandleSendRaceFacts` | Returns the latest-session race control events. | Used for yellow flag, SC, incidents, etc. |
| `HandleSendDriverRace` | Returns all latest-session datasets for one driver chosen by query param. | Compact per-driver payload for AR focus modes. |
| `HandleSendRaceStandings` | Computes in-session standings snapshots from position data. | Used for live/replay standings at any time. |
| `HandleSendVideoURL` | Returns a placeholder broadcast URL for the latest stored session. | Legacy convenience endpoint for the active session view. |
| `HandleSendMetadata` | Returns meeting/session metadata for the latest stored session. | Lets clients sync labels and time context. |
| `HandleListDatasets` | Lists available public datasets and their counts. | Useful for discovery and debugging. |
| `HandleSendDataset` | Returns one whole dataset by name. | Generic endpoint for consumers that know what they need. |
| `HandleListDrivers` | Returns the driver roster. | Basic entrypoint to choose a driver. |
| `HandleListTeams` | Returns derived team list from drivers data. | Needed for team-centric UIs. |
| `HandleSendDriverRaceResource` | Returns the full per-driver payload via path param. | Cleaner REST alternative to query-string driver access. |
| `HandleSendDriverProfile` | Returns only the driver profile row. | Lightweight driver info endpoint. |
| `HandleSendDriverDataset` | Returns one dataset filtered to one driver. | Fine-grained AR/mobile fetching. |

#### Handler internals

| Function | Purpose | Why it exists |
|---|---|---|
| `parseInput` | Builds `RaceBuildInput` from query params plus defaults. | Keeps `HandleGetRace` small and validates input early. |
| `getStoredArchive` | Reads the latest raw archive or writes an HTTP error. | Internal helper for endpoints that need raw storage access. |
| `getMergedArchive` | Reads the latest merged archive or writes an HTTP error. | Internal helper used by almost all read endpoints. |
| `getSessionArchive` | Reads the merged archive for one explicit session or writes an HTTP error. | Internal helper used by session-scoped endpoints. |
| `parseDriverNumberForRace` | Resolves `driver_number` from query string or stored metadata. | Supports both explicit and default driver workflows. |
| `writeDriverRacePayload` | Builds and writes the full per-driver response. | Centralizes the per-driver payload shape. |
| `parsePathDriverNumber` | Parses driver number from route path. | Used by `/drivers/{driverNumber}/...` endpoints. |
| `parseSnapshotTime` | Parses the optional `at` timestamp for replay/standings. | Allows time-travel queries in standings endpoint. |
| `buildRaceStandings` | Builds one latest position row per driver at a given time. | Core logic behind replay/live standings snapshots. |
| `indexDriversByNumber` | Creates a map of `driver_number -> driver row`. | Used to enrich standings rows with driver info. |
| `readTimeField` | Reads time fields from a row. | Handles mixed loose row formats. |
| `cloneRow` | Shallow-copies a row map. | Avoids mutating stored rows before enriching responses. |
| `sortDriverRows` | Sorts drivers by number. | Makes driver list output stable. |
| `buildTeams` | Builds unique team rows from driver rows. | There is no direct `teams` dataset from OpenF1 in this archive shape. |
| `filterRowsByDriver` | Keeps rows matching one `driver_number`. | Reused across per-driver endpoints. |
| `resolveDatasetName` | Maps public aliases like `telemetry` or `facts` to stored dataset keys. | Gives cleaner API naming without changing storage keys. |
| `datasetPathValue` | Resolves dataset names from routed path values or the final path segment. | Keeps concrete driver dataset routes and generic dataset routes aligned. |
| `readStringValue` | Reads string-like values from a row. | Handles weakly typed `map[string]any` rows. |
| `readIntField` | Reads int-like values from a row. | Same reason as above for numeric fields. |

## Prisma Repository

### `internal/repository/prisma/`

The Prisma repository is split by responsibility. One file owns the write entrypoints, one owns catalog reads, one owns metadata upserts, one owns participant/raw chunk storage, one owns normalized table writes, and one centralizes small helpers.

### Repository Call Graph

When `RaceService.FetchAndStore` persists an archive, the repository flow is:

1. `NewRaceArchiveStore` builds the repository object.
2. `Store` orchestrates the write.
3. `ensureProvider` -> `ensureChampionship` -> `ensureEvent` -> `ensureSession`
4. `syncParticipants`
5. `buildDatasetChunkQueries`
6. `storeNormalizedSessionDataTx`
7. one Prisma transaction commits archive row, raw chunks, and normalized tables together

When a read endpoint asks for stored race data, the repository flow is:

1. `GetLatest`, `GetLatestMerged`, `GetSessionMerged`, `GetSession`, or `GetSessionDriverBroadcast`
2. `archiveFromModel`, `mergeArchiveModels`, or direct catalog/broadcast lookup
3. handlers serialize the rebuilt domain archive

### [`race_archive_store.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_store.go)

This file contains the repository entrypoints and the high-level store flow.

| Function | Purpose | Why it exists |
|---|---|---|
| `NewRaceArchiveStore` | Creates the Prisma-backed repository. | Injects the Prisma client into the storage layer. |
| `(*RaceArchiveStore).Store` | Persists one archive, its raw chunked datasets, and normalized rows. | Main DB write entrypoint used by `GET /getrace`. |
| `(*RaceArchiveStore).GetLatest` | Returns the latest raw stored archive. | Used for status and legacy raw reads. |
| `(*RaceArchiveStore).GetLatestMerged` | Rebuilds a merged archive for the latest session. | Prevents losing prior driver-focused imports on read. |
| `(*RaceArchiveStore).GetSessionMerged` | Rebuilds a merged archive for one explicit session. | Powers historical reads without relying on the latest session pointer. |

### [`race_archive_catalog.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_catalog.go)

This file owns all read-side catalog queries and archive reconstruction logic.

| Function | Purpose | Why it exists |
|---|---|---|
| `(*RaceArchiveStore).ListChampionships` | Lists stored championships. | Entry point for `championship -> event` browsing. |
| `(*RaceArchiveStore).GetChampionshipEvents` | Returns one championship with its events. | Powers championship catalog endpoints. |
| `(*RaceArchiveStore).GetEvent` | Returns one event summary. | Powers event catalog detail endpoints. |
| `(*RaceArchiveStore).ListEventSessions` | Returns sessions of one event. | Needed because an event owns multiple sessions. |
| `(*RaceArchiveStore).GetSession` | Returns one session summary. | Powers session catalog detail endpoints. |
| `(*RaceArchiveStore).GetSessionDriverBroadcast` | Returns one stored driver broadcast URL. | Powers explicit driver/session broadcast lookup. |
| `(*RaceArchiveStore).archiveFromModel` | Rebuilds `domain.RaceArchive` from one archive row and its chunks. | Converts DB storage back into domain payload. |
| `(*RaceArchiveStore).mergeArchiveModels` | Merges several archive rows from the same session into one logical archive. | Critical when imports are done driver by driver. |
| `championshipSummaryFromModel` | Maps Prisma championship model to domain summary. | Keeps the API model decoupled from Prisma internals. |
| `eventSummaryFromModel` | Maps Prisma event model to domain summary. | Same reason. |
| `(*RaceArchiveStore).sessionSummaryFromModel` | Maps Prisma session model to domain summary. | Same reason. |

### [`race_archive_entities.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_entities.go)

This file owns top-level metadata upserts before any race payload is written.

| Function | Purpose | Why it exists |
|---|---|---|
| `(*RaceArchiveStore).ensureProvider` | Ensures the OpenF1 provider row exists. | Keeps provider metadata stable and reusable. |
| `(*RaceArchiveStore).ensureChampionship` | Ensures the `f1` championship exists. | Creates the `championship` root node for the catalog. |
| `(*RaceArchiveStore).ensureEvent` | Ensures the event row exists for the imported archive. | Materializes the `championship -> event` relation from provider meeting data. |
| `(*RaceArchiveStore).ensureSession` | Ensures the session row exists. | Materializes the `event -> session` relation. |

### [`race_archive_participants.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_participants.go)

This file owns raw chunk persistence and team/driver synchronization.

| Function | Purpose | Why it exists |
|---|---|---|
| `(*RaceArchiveStore).storeDatasets` | Persists raw datasets into `RaceDatasetChunk`. | Direct helper when raw archive chunks need to be written outside the full transactional path. |
| `(*RaceArchiveStore).buildDatasetChunkQueries` | Converts raw datasets into chunked Prisma transaction queries. | Prevents huge single-row JSON blobs in `RaceDatasetChunk`. |
| `(*RaceArchiveStore).syncParticipants` | Synchronizes teams and drivers from the `drivers` dataset. | Ensures participant entities exist before normalized writes. |
| `(*RaceArchiveStore).ensureSessionDriverBroadcasts` | Upserts one broadcast URL per driver for the current session. | Populates session-scoped driver broadcast storage automatically during import. |
| `(*RaceArchiveStore).ensureTeam` | Upserts one team inferred from driver data. | Keeps team rows consistent across imports. |
| `(*RaceArchiveStore).ensureDriver` | Upserts one driver inferred from driver data. | Keeps driver rows consistent across imports. |

### [`race_archive_normalized.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_normalized.go)

This file owns cleanup and normalized inserts into race-specific tables.

#### Coordinators

| Function | Purpose | Why it exists |
|---|---|---|
| `(*RaceArchiveStore).storeNormalizedSessionData` | Writes normalized rows directly for one session. | Non-transactional helper path kept for repository-level reuse. |
| `(*RaceArchiveStore).storeNormalizedSessionDataTx` | Builds the normalized write batch for one session. | Used by `Store` so archive + chunks + normalized rows commit atomically. |

#### Cleanup helpers

| Function | Purpose | Why it exists |
|---|---|---|
| `(*RaceArchiveStore).clearSessionData` | Deletes all normalized rows for one session. | Used before replacing a full-session import. |
| `(*RaceArchiveStore).clearDriverSessionData` | Deletes normalized rows for one driver in one session. | Used before replacing a driver-focused import. |
| `(*RaceArchiveStore).clearSharedSessionData` | Deletes shared session datasets when refreshed. | Avoids duplicating weather, race control, and similar shared rows. |
| `(*RaceArchiveStore).clearSessionDataTx` | Builds transaction deletes for full-session refresh. | Transactional equivalent of `clearSessionData`. |
| `(*RaceArchiveStore).clearDriverSessionDataTx` | Builds transaction deletes for one driver refresh. | Transactional equivalent of `clearDriverSessionData`. |
| `(*RaceArchiveStore).clearSharedSessionDataTx` | Builds transaction deletes for shared datasets. | Transactional equivalent of `clearSharedSessionData`. |

#### Dataset insertors

| Function | Purpose | Why it exists |
|---|---|---|
| `insertRaceLaps` | Inserts `laps` into `RaceLap`. | Normalized lap storage. |
| `insertRaceTelemetry` | Inserts `car_data` into `RaceTelemetrySample`. | Normalized telemetry storage. |
| `insertRaceLocations` | Inserts `location` into `RaceLocationSample`. | Normalized location storage. |
| `insertRacePositions` | Inserts `position` into `RacePositionSample`. | Normalized position storage. |
| `insertRaceIntervals` | Inserts `intervals` into `RaceIntervalSample`. | Normalized gap storage. |
| `insertRaceStints` | Inserts `stints` into `RaceStint`. | Normalized tire strategy storage. |
| `insertRacePitStops` | Inserts `pit` into `RacePitStop`. | Normalized pit storage. |
| `insertRaceControlEvents` | Inserts `race_control` into `RaceControlEvent`. | Normalized race facts storage. |
| `insertTeamRadioMessages` | Inserts `team_radio` into `TeamRadioMessage`. | Normalized team radio storage. |
| `insertRaceWeatherSamples` | Inserts `weather` into `RaceWeatherSample`. | Normalized weather storage. |
| `insertSessionResults` | Inserts `session_result` into `SessionResultRow`. | Normalized result storage. |
| `insertStartingGridRows` | Inserts `starting_grid` into `StartingGridRow`. | Normalized grid storage. |
| `insertOvertakeEvents` | Inserts `overtakes` into `OvertakeEvent`. | Normalized overtake storage. |
| `insertDriverChampionshipRows` | Inserts `championship_drivers`. | Normalized driver standings storage. |
| `insertTeamChampionshipRows` | Inserts `championship_teams`. | Normalized constructor standings storage. |
| `executeDatasetInsert` | Runs one raw SQL JSON insert immediately. | Shared low-level insert helper for direct writes. |
| `executeDatasetInsertTx` | Builds one raw SQL JSON insert transaction. | Shared low-level insert helper for atomic writes. |

### [`race_archive_helpers.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_helpers.go)

This file centralizes the small reusable helpers used across all repository files.

#### Archive/status inference

| Function | Purpose | Why it exists |
|---|---|---|
| `inferEventBounds` | Infers temporal bounds from archive rows. | Gives event and session rows consistent start and end values. |
| `resolveArchiveMode` | Resolves full vs driver-focused import mode. | Persists the archive import strategy. |
| `resolveEventStatus` | Resolves event lifecycle status. | Normalizes provider data into DB enum values. |
| `resolveSessionType` | Resolves session type from session name. | Maps provider naming to DB enum values. |
| `resolveSessionStatus` | Resolves session lifecycle status. | Normalizes provider data into DB enum values. |
| `sessionBroadcastURL` | Builds the placeholder broadcast URL stored on a session. | Centralizes the session-level broadcast format. |
| `sessionDriverBroadcastURL` | Builds the placeholder broadcast URL stored per driver/session. | Centralizes the driver-level broadcast format. |

#### JSON helpers

| Function | Purpose | Why it exists |
|---|---|---|
| `marshalPrismaJSON` | Marshals Go values into Prisma JSON. | Used for archive metadata, counts, errors, and raw rows. |
| `unmarshalPrismaJSON` | Unmarshals Prisma JSON into Go values. | Used when rebuilding an archive from DB. |
| `unmarshalMetadataEnvelope` | Unmarshals the metadata envelope. | Special helper for archive metadata. |

#### Row readers and small normalizers

| Function | Purpose | Why it exists |
|---|---|---|
| `firstTimeFromRows` | Reads the first valid time from a row. | Helps infer temporal bounds from provider rows. |
| `readOptionalStringField` | Reads a nullable string pointer from a row. | Used by Prisma setters and nullable DB columns. |
| `readOptionalIntField` | Reads a nullable int pointer from a row. | Same reason for numeric columns. |
| `fallbackString` | Returns a fallback when a string is empty. | Small normalization helper. |
| `nonNilMap` | Ensures a map is never nil. | Keeps JSON serialization stable. |
| `nonNilRows` | Ensures a row slice is never nil. | Same reason for row arrays. |
| `optionalString` | Returns a string pointer when non-empty. | Small Prisma setter helper. |
| `optionalInt` | Returns an int pointer when non-zero. | Same reason. |
| `optionalTime` | Returns a time pointer when non-zero. | Same reason. |
| `optionalStringValue` | Normalizes optional strings before storage. | Avoids empty garbage values. |
| `optionalIntPointer` | Returns a pointer for positive ints. | Small setter helper. |
| `cloneMap` | Copies a row map. | Prevents accidental shared mutation. |
| `mergeUniqueRows` | Merges row slices while deduplicating. | Used when rebuilding merged archives. |
| `rowSignature` | Builds a stable row identity. | Powers deduplication in merged reads. |
| `readIntField` | Reads int-like values from weakly typed rows. | Handles `map[string]any` provider data. |
| `readStringField` | Reads string-like values from weakly typed rows. | Same reason. |
| `readFirstStringField` | Reads the first non-empty value across several keys. | Handles alternate provider field names. |
| `teamExternalKey` | Builds a stable synthetic team key. | Needed because OpenF1 does not always provide one. |
| `normalizeColor` | Normalizes team colors. | Keeps color storage consistent. |
| `chunkRows` | Splits raw rows into deterministic chunks. | Used to populate `RaceDatasetChunk` safely. |
| `newUUID` | Generates an archive UUID before commit. | Lets one transaction reference archive and chunk rows together. |

## Scripts

### `scripts/run_backend.sh`

| Function | Purpose | Why it exists |
|---|---|---|
| `main` | Starts the local backend with the local PostgreSQL convention used in development. | Simple tracked helper for local runtime. |

### `scripts/run_prisma.sh`

| Function | Purpose | Why it exists |
|---|---|---|
| `main` | Starts Prisma Studio against the local database. | Simple tracked helper for local DB inspection. |

## Practical Mental Model

If you want to understand the backend quickly, keep this map in mind:

1. `main` starts everything
2. `NewHTTPServer` wires dependencies
3. `NewRouter` exposes HTTP routes
4. `RaceHandler` translates HTTP <-> service calls
5. `RaceService` orchestrates business operations
6. `RaceBuilder` talks to OpenF1 and builds archive payloads
7. `RaceArchiveStore` stores raw + normalized race data in PostgreSQL
8. read handlers then serve merged data back to AR/mobile/web clients

## Suggested Reading Order

1. [`cmd/api/main.go`](/home/bastou/delivery/eip/OD_Backend/cmd/api/main.go)
2. [`internal/app/server.go`](/home/bastou/delivery/eip/OD_Backend/internal/app/server.go)
3. [`internal/api/router.go`](/home/bastou/delivery/eip/OD_Backend/internal/api/router.go)
4. [`internal/api/race_handler.go`](/home/bastou/delivery/eip/OD_Backend/internal/api/race_handler.go)
5. [`internal/service/race_service.go`](/home/bastou/delivery/eip/OD_Backend/internal/service/race_service.go)
6. [`internal/usecase/build_race.go`](/home/bastou/delivery/eip/OD_Backend/internal/usecase/build_race.go)
7. [`internal/repository/prisma/race_archive_store.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_store.go)
8. [`internal/repository/prisma/race_archive_catalog.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_catalog.go)
9. [`internal/repository/prisma/race_archive_entities.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_entities.go)
10. [`internal/repository/prisma/race_archive_participants.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_participants.go)
11. [`internal/repository/prisma/race_archive_normalized.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_normalized.go)
12. [`internal/repository/prisma/race_archive_helpers.go`](/home/bastou/delivery/eip/OD_Backend/internal/repository/prisma/race_archive_helpers.go)
13. [`internal/providers/openf1/client.go`](/home/bastou/delivery/eip/OD_Backend/internal/providers/openf1/client.go)
