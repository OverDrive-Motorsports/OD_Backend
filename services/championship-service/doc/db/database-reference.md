# championship-service database reference

## Overview

This database stores championship catalog data, competition structure, event scheduling, session metadata, and standings snapshots for `championship-service`.

## Enum reference

### `ProviderStatus`

| Value | Description | Example usage |
| --- | --- | --- |
| `active` | Provider is fully enabled in the platform | Main production provider available to users |
| `beta` | Provider is partially available or still under validation | New provider tested on a limited set of championships |
| `disabled` | Provider is not currently usable | Temporarily disabled upstream integration |

### `EventStatus`

| Value | Description | Example usage |
| --- | --- | --- |
| `scheduled` | Event is planned but not started yet | A Grand Prix planned for next Sunday |
| `live` | Event is currently ongoing | A race weekend currently in progress |
| `finished` | Event is completed | A race weekend that ended yesterday |

### `SessionType`

| Value | Description | Example usage |
| --- | --- | --- |
| `race` | Main race session | Sunday Grand Prix |
| `sprint` | Sprint race session | Saturday sprint race |
| `quali` | Qualifying session | Friday or Saturday qualifying |
| `practice` | Free practice session | FP1 session |

### `SessionStatus`

| Value | Description | Example usage |
| --- | --- | --- |
| `scheduled` | Session has not started yet | Upcoming qualifying session |
| `live` | Session is currently running | Live race session |
| `finished` | Session is over | Completed practice session |

## Relationships

- `Provider` `1 -> N` `Championship`
- `Championship` `1 -> N` `Event`
- `Championship` `1 -> N` `Team`
- `Championship` `1 -> N` `Driver`
- `Event` `1 -> N` `Session`
- `Team` `1 -> N` `Driver`
- `Session` `1 -> N` `SessionResultRow`
- `Session` `1 -> N` `StartingGridRow`
- `Session` `1 -> N` `DriverChampionshipStanding`
- `Session` `1 -> N` `TeamChampionshipStanding`

## Entity reference

### `Provider`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the provider | `3ad14f5a-61e0-4b41-a2f1-7fd4d2dbaf10` |
| `code` | `String` | Yes | Unique, `varchar(20)` | Stable business code of the provider | `f1tv` |
| `name` | `String` | Yes | `varchar(50)` | Human-readable provider name | `Formula 1 TV` |
| `status` | `ProviderStatus` | Yes | Default `active` | Operational state of the provider | `active` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp of the provider record | `2026-04-14T08:00:00Z` |

### `Championship`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the championship | `e7af5e61-88dc-45b9-8dc2-cb9a6f2da501` |
| `providerId` | `String` | Yes | Foreign key to `Provider.id`, indexed | Provider owning the championship | `3ad14f5a-61e0-4b41-a2f1-7fd4d2dbaf10` |
| `code` | `String` | Yes | `varchar(20)` | Provider-scoped code of the championship | `f1` |
| `name` | `String` | Yes | `varchar(80)` | Display name of the championship | `Formula 1 World Championship` |
| `category` | `String?` | No | `varchar(40)` | Optional classification or category label | `single-seater` |
| `isActive` | `Boolean` | Yes | Default `true` | Indicates whether the championship is active in the platform | `true` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-04-14T08:05:00Z` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last update timestamp | `2026-04-14T09:00:00Z` |

### `Event`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the event | `0c48a61a-bf67-4f92-93fe-50c6d74a4c31` |
| `championshipId` | `String` | Yes | Foreign key to `Championship.id`, indexed | Championship owning the event | `e7af5e61-88dc-45b9-8dc2-cb9a6f2da501` |
| `seasonYear` | `Int` | Yes | Indexed with `startTimeUtc` | Season year of the event | `2026` |
| `roundNumber` | `Int?` | No | None | Optional round number in the season calendar | `5` |
| `name` | `String` | Yes | `varchar(120)` | Short name of the event | `Miami Grand Prix` |
| `officialName` | `String?` | No | `varchar(255)` | Full official event name | `Formula 1 Crypto.com Miami Grand Prix 2026` |
| `location` | `String` | Yes | `varchar(255)` | Main location of the event | `Miami` |
| `countryName` | `String?` | No | `varchar(80)` | Country of the event | `United States` |
| `countryCode` | `String?` | No | `varchar(3)` | ISO-like short country code | `USA` |
| `circuitName` | `String?` | No | `varchar(120)` | Name of the circuit | `Miami International Autodrome` |
| `externalKey` | `String?` | No | `varchar(64)` | Optional provider-side identifier of the event | `miami-2026` |
| `startTimeUtc` | `DateTime` | Yes | None | Event start timestamp in UTC | `2026-05-01T14:00:00Z` |
| `endTimeUtc` | `DateTime` | Yes | None | Event end timestamp in UTC | `2026-05-03T20:00:00Z` |
| `status` | `EventStatus` | Yes | None | Lifecycle status of the event | `scheduled` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-04-14T08:10:00Z` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last update timestamp | `2026-04-14T08:30:00Z` |

### `Session`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the session | `f502257b-8ce5-46f4-843e-22f84a1e2797` |
| `eventId` | `String` | Yes | Foreign key to `Event.id`, indexed | Event owning the session | `0c48a61a-bf67-4f92-93fe-50c6d74a4c31` |
| `type` | `SessionType` | Yes | Enum | Functional type of the session | `race` |
| `status` | `SessionStatus` | Yes | Enum | Lifecycle status of the session | `scheduled` |
| `name` | `String?` | No | `varchar(80)` | Optional display label of the session | `Grand Prix` |
| `externalKey` | `String?` | No | `varchar(64)` | Optional provider-side identifier of the session | `race-main` |
| `broadcastUrl` | `String?` | No | `varchar(255)` | Optional stream or broadcast link | `https://stream.example.com/f1/miami/race` |
| `startedAtUtc` | `DateTime` | Yes | None | Planned or actual start time in UTC | `2026-05-03T19:00:00Z` |
| `endedAtUtc` | `DateTime?` | No | None | Planned or actual end time in UTC | `2026-05-03T21:00:00Z` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-04-14T08:15:00Z` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last update timestamp | `2026-04-14T08:45:00Z` |

### `Team`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the team | `30c0cf06-5915-43f6-8f40-a4fe50df3b71` |
| `championshipId` | `String` | Yes | Foreign key to `Championship.id`, indexed | Championship owning the team | `e7af5e61-88dc-45b9-8dc2-cb9a6f2da501` |
| `name` | `String` | Yes | `varchar(80)` | Team display name | `Red Bull Racing` |
| `code` | `String?` | No | `varchar(20)` | Optional short team code | `RBR` |
| `colorHex` | `String?` | No | `varchar(7)` | Optional team color in hex format | `#1E41FF` |
| `externalKey` | `String?` | No | `varchar(64)` | Optional provider-side identifier | `team-rbr` |

### `Driver`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the driver | `f9a76cbd-4067-41a0-a4b5-df15f2447fcd` |
| `championshipId` | `String` | Yes | Foreign key to `Championship.id`, indexed | Championship of the driver | `e7af5e61-88dc-45b9-8dc2-cb9a6f2da501` |
| `teamId` | `String` | Yes | Foreign key to `Team.id`, indexed | Current team of the driver | `30c0cf06-5915-43f6-8f40-a4fe50df3b71` |
| `displayName` | `String` | Yes | `varchar(80)` | Full display name of the driver | `Max Verstappen` |
| `firstName` | `String?` | No | `varchar(60)` | Optional first name | `Max` |
| `lastName` | `String?` | No | `varchar(60)` | Optional last name | `Verstappen` |
| `code` | `String?` | No | `varchar(20)` | Optional short driver code | `VER` |
| `number` | `Int` | Yes | None | Racing number of the driver | `1` |
| `countryCode` | `String?` | No | `varchar(3)` | Driver nationality code | `NLD` |
| `externalKey` | `String?` | No | `varchar(64)` | Optional provider-side identifier | `driver-1` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-04-14T08:20:00Z` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last update timestamp | `2026-04-14T08:50:00Z` |

### `SessionResultRow`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the result row | `1` |
| `sessionId` | `String` | Yes | Foreign key to `Session.id` | Session owning the result row | `f502257b-8ce5-46f4-843e-22f84a1e2797` |
| `driverNumber` | `Int` | Yes | Unique with `sessionId` | Driver race number | `1` |
| `position` | `Int?` | No | Indexed with `sessionId` | Final or current position in the session | `1` |
| `points` | `Float?` | No | None | Points awarded for the session result | `25.0` |
| `status` | `String?` | No | `varchar(40)` | Classification status or finish reason | `Finished` |
| `raw` | `Json` | Yes | None | Raw provider payload kept for traceability | `{"classified":true,"laps":57}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T21:05:00Z` |

### `StartingGridRow`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the grid row | `1` |
| `sessionId` | `String` | Yes | Foreign key to `Session.id` | Session owning the grid row | `f502257b-8ce5-46f4-843e-22f84a1e2797` |
| `driverNumber` | `Int` | Yes | Unique with `sessionId` | Driver race number | `1` |
| `gridPosition` | `Int?` | No | Indexed with `sessionId` | Starting position on the grid | `1` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"q3Time":"1:27.241"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T18:00:00Z` |

### `DriverChampionshipStanding`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the driver standings row | `1` |
| `sessionId` | `String` | Yes | Foreign key to `Session.id` | Session snapshot owner | `f502257b-8ce5-46f4-843e-22f84a1e2797` |
| `driverNumber` | `Int` | Yes | Unique with `sessionId` | Driver race number | `1` |
| `positionCurrent` | `Int?` | No | Indexed with `sessionId` | Driver rank after the session | `1` |
| `positionStart` | `Int?` | No | None | Driver rank before the session | `2` |
| `pointsCurrent` | `Float?` | No | None | Driver points after the session | `110.0` |
| `pointsStart` | `Float?` | No | None | Driver points before the session | `85.0` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"delta":"+1","wins":3}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T21:10:00Z` |

### `TeamChampionshipStanding`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the team standings row | `1` |
| `sessionId` | `String` | Yes | Foreign key to `Session.id` | Session snapshot owner | `f502257b-8ce5-46f4-843e-22f84a1e2797` |
| `teamName` | `String?` | No | `varchar(80)`, indexed with `sessionId` | Team display name | `Red Bull Racing` |
| `teamCode` | `String?` | No | `varchar(20)` | Optional short team code | `RBR` |
| `positionCurrent` | `Int?` | No | Indexed with `sessionId` | Team rank after the session | `1` |
| `positionStart` | `Int?` | No | None | Team rank before the session | `1` |
| `pointsCurrent` | `Float?` | No | None | Team points after the session | `210.0` |
| `pointsStart` | `Float?` | No | None | Team points before the session | `185.0` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"podiums":6}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T21:12:00Z` |

## Constraints and indexes

| Item | Type | Description |
| --- | --- | --- |
| `Provider.code` | Unique constraint | Prevents duplicate providers |
| `Championship(providerId, code)` | Unique constraint | Prevents duplicate championship codes inside a provider |
| `Event(championshipId, externalKey)` | Unique constraint | Prevents duplicate provider event references inside a championship |
| `Session(eventId, externalKey)` | Unique constraint | Prevents duplicate provider session references inside an event |
| `Team(championshipId, externalKey)` | Unique constraint | Prevents duplicate provider team references inside a championship |
| `Team(championshipId, code)` | Unique constraint | Prevents duplicate team codes inside a championship |
| `Driver(championshipId, code)` | Unique constraint | Prevents duplicate driver codes inside a championship |
| `Driver(championshipId, externalKey)` | Unique constraint | Prevents duplicate provider driver references inside a championship |
| `SessionResultRow(sessionId, driverNumber)` | Unique constraint | One result row per driver and session |
| `StartingGridRow(sessionId, driverNumber)` | Unique constraint | One grid row per driver and session |
| `DriverChampionshipStanding(sessionId, driverNumber)` | Unique constraint | One driver standing snapshot per driver and session |
| `providerId`, `championshipId`, `eventId`, `teamId` | Indexes | Speed up parent-child reads |
| `(seasonYear, startTimeUtc)` | Composite index | Speeds up calendar navigation by season and date |
| `(sessionId, position)` style indexes | Composite indexes | Speed up ranking and ordering queries |
