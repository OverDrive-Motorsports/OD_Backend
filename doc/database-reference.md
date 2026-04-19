# backend database reference

## Overview

This document groups all database schemas used in the backend monorepo.
At the moment, 4 services own a database:

| Service | Main role | Local database documentation |
| --- | --- | --- |
| `auth-service` | Authentication and refresh sessions | `services/auth-service/doc/db/database-reference.md` |
| `user-data-service` | User preferences, provider links, saved layouts | `services/user-data-service/doc/db/database-reference.md` |
| `championship-service` | Championship catalog, calendar, participants, standings | `services/championship-service/doc/db/database-reference.md` |
| `race-data-service` | Telemetry, race timeline, media data, dataset archives | `services/race-data-service/doc/db/database-reference.md` |

<br>
<br>

## Global architecture

Each service owns its schema independently.
There are no direct SQL foreign keys between services.
Cross-service consistency is handled through shared business identifiers stored as plain fields.
Each service must also keep an isolated physical database connection in deployment and local development.

<br>
<br>

## Database inventory

### `auth-service` database

| Item | Value |
| --- | --- |
| Main purpose | Store platform users and refresh-token sessions |
| Core entities | `User`, `AuthSession` |
| Strong relationship | `User 1 -> N AuthSession` |
| Main business identifiers | `User.id`, `User.email` |
| Main technical pattern | Small transactional schema with strict ownership and cascade delete |

### `user-data-service` database

| Item | Value |
| --- | --- |
| Main purpose | Store user-scoped provider connections and saved layouts |
| Core entities | `ProviderConnection`, `UserPreset`, `PresetWidget` |
| Strong relationship | `UserPreset 1 -> N PresetWidget` |
| Main business identifiers | `userId`, `providerId`, `UserPreset.id` |
| Main technical pattern | User configuration schema with JSON settings and layout coordinates |

### `championship-service` database

| Item | Value |
| --- | --- |
| Main purpose | Store competition catalog, events, sessions, teams, drivers, and standings snapshots |
| Core entities | `Provider`, `Championship`, `Event`, `Session`, `Team`, `Driver`, standings tables |
| Strong relationships | `Provider -> Championship -> Event -> Session` and `Championship -> Team -> Driver` |
| Main business identifiers | `providerId`, `championshipId`, `eventId`, `sessionId`, `teamId`, `driverNumber` |
| Main technical pattern | Structured domain schema with catalog entities plus session-level analytical snapshots |

### `race-data-service` database

| Item | Value |
| --- | --- |
| Main purpose | Store high-volume race read models, telemetry, incidents, media data, and archive exports |
| Core entities | `RaceLap`, `RaceTelemetrySample`, `RaceLocationSample`, `RacePositionSample`, `RaceArchive`, `RaceDatasetChunk`, others |
| Strong relationship | `RaceArchive 1 -> N RaceDatasetChunk` |
| Main business identifiers | `sessionId`, `driverNumber`, `dateUtc`, `archiveId` |
| Main technical pattern | Time-series and append-heavy schema optimized for read models and archive generation |

<br>
<br>

## Cross-service logical links

The databases are independent, but several fields clearly act as logical bridges between services.

| Source service | Field | Target service | Meaning | Example |
| --- | --- | --- | --- | --- |
| `user-data-service` | `ProviderConnection.userId` | `auth-service` | Refers to the authenticated platform user | `550e8400-e29b-41d4-a716-446655440000` |
| `user-data-service` | `ProviderConnection.providerId` | `championship-service` | Refers logically to a provider namespace or upstream source | `f1tv` |
| `race-data-service` | `sessionId` | `championship-service` | Refers logically to a race session defined in the competition catalog | `session_2026_miami_race` |
| `race-data-service` | `driverId` in `SessionDriverBroadcast` | `championship-service` | Refers logically to a driver entity managed in the championship domain | `driver_1` |
| `race-data-service` | `driverNumber` | `championship-service` | Refers to the race number used by standings and session data | `1` |

<br>
<br>

## Common schema patterns

These patterns are repeated across multiple independent databases.
They do not mean that the services share one physical database.

| Pattern | Services using it | Description | Example |
| --- | --- | --- | --- |
| UUID primary keys | `auth-service`, `user-data-service`, `championship-service`, part of `race-data-service` | Stable identifiers for business entities | `98c9f3fd-5305-4af5-b5c0-790c8fe3d4ff` |
| Auto-increment `BigInt` primary keys | `championship-service`, `race-data-service` | Efficient storage for high-volume snapshot or sample rows | `45001` |
| JSON payload columns | `user-data-service`, `championship-service`, `race-data-service` | Preserve provider payloads or dynamic configuration | `{"stream":"onboard","driverNumber":1}` |
| `createdAt` timestamps | All 4 services | Track insertion time | `2026-04-14T10:00:00Z` |
| `updatedAt` timestamps | `auth-service`, `user-data-service`, `championship-service`, part of `race-data-service` | Track record updates | `2026-04-14T10:15:00Z` |
| Composite uniqueness | All 4 services | Prevent duplicate business rows in scoped contexts | `(sessionId, driverNumber, lapNumber)` |

<br>
<br>

## Domain boundaries

### Authentication boundary

`auth-service` owns user identity and refresh-session lifecycle.
Other services may reuse the user identifier logically, but they do not own authentication data.

### User personalization boundary

`user-data-service` owns mutable user preferences, saved presets, and provider connection settings.
It should not duplicate authentication data or championship catalog data.

### Competition catalog boundary

`championship-service` owns the reference structure of the sport domain:

- providers
- championships
- events
- sessions
- teams
- drivers
- standings snapshots

This service acts as the most natural catalog reference for `sessionId`, `teamId`, and driver-oriented identifiers used elsewhere.

### Race analytics boundary

`race-data-service` owns dense race execution data:

- laps
- telemetry
- positions
- intervals
- weather
- pit stops
- race control events
- highlights
- archives

It is optimized for read-heavy and timeline-heavy usage rather than master-data ownership.

<br>
<br>

## Recommended reading order

| Need | Document to open |
| --- | --- |
| Understand auth storage | `services/auth-service/doc/db/database-reference.md` |
| Understand user presets and settings | `services/user-data-service/doc/db/database-reference.md` |
| Understand sports catalog and standings | `services/championship-service/doc/db/database-reference.md` |
| Understand telemetry and race datasets | `services/race-data-service/doc/db/database-reference.md` |

<br>
<br>

## Summary

The backend currently uses a distributed database model:

- one database per service
- no direct relational coupling between services
- logical consistency maintained through shared identifiers
- strong separation between transactional data, reference catalog data, and analytical race data

## Configuration rules

The database split is enforced by configuration as well as by schema design.

| Service | Prisma schema | Environment variable | Default local database |
| --- | --- | --- | --- |
| `auth-service` | `services/auth-service/resources/schema.prisma` | `AUTH_DATABASE_URL` | `overdrive_auth` |
| `user-data-service` | `services/user-data-service/resources/schema.prisma` | `USER_DATA_DATABASE_URL` | `overdrive_user_data` |
| `championship-service` | `services/championship-service/resources/schema.prisma` | `CHAMPIONSHIP_DATABASE_URL` | `overdrive_championship` |
| `race-data-service` | `services/race-data-service/resources/schema.prisma` | `RACE_DATA_DATABASE_URL` | `overdrive_race_data` |

When Prisma is executed from the repository root, it loads the `.env` file from the owning service directory before resolving the datasource URL.
If a service-specific environment variable is missing, the local fallback still targets that service's own database rather than a shared `overdrive` database.
