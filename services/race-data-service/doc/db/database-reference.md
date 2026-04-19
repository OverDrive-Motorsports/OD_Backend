# race-data-service database reference

## Overview

This database stores normalized race read models, telemetry samples, race events, media-oriented data, and archived export datasets for `race-data-service`.

## Enum reference

### `ArchiveMode`

| Value | Description | Example usage |
| --- | --- | --- |
| `full` | Archive generated for the complete session dataset | Export all race datasets for a Grand Prix |
| `driver_focused` | Archive generated for one specific driver view | Export only telemetry and incidents for driver `1` |

## Data domains

- timing and telemetry
- race flow and incidents
- highlights and broadcast
- archived datasets

## Relationship

- `RaceArchive` `1 -> N` `RaceDatasetChunk`

All other tables use `sessionId` as a logical link without an enforced foreign key in this schema.

## Entity reference

### `RaceLap`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the lap row | `1` |
| `sessionId` | `String` | Yes | Unique with `driverNumber` and `lapNumber`, indexed | Session identifier | `session_2026_miami_race` |
| `driverNumber` | `Int` | Yes | Unique with `sessionId` and `lapNumber`, indexed | Driver race number | `1` |
| `lapNumber` | `Int` | Yes | Unique with `sessionId` and `driverNumber`, indexed | Lap number in the session | `27` |
| `dateStartUtc` | `DateTime?` | No | Indexed with `sessionId` | Start time of the lap | `2026-05-03T19:41:12Z` |
| `lapDurationSec` | `Float?` | No | None | Total lap duration in seconds | `87.241` |
| `sector1Sec` | `Float?` | No | None | Sector 1 duration in seconds | `28.153` |
| `sector2Sec` | `Float?` | No | None | Sector 2 duration in seconds | `29.012` |
| `sector3Sec` | `Float?` | No | None | Sector 3 duration in seconds | `30.076` |
| `speedI1Kph` | `Int?` | No | None | Speed at intermediate point 1 in km/h | `286` |
| `speedI2Kph` | `Int?` | No | None | Speed at intermediate point 2 in km/h | `301` |
| `speedTrapKph` | `Int?` | No | None | Speed trap value in km/h | `329` |
| `isPitOutLap` | `Boolean?` | No | None | Indicates whether the lap started from pit exit | `false` |
| `raw` | `Json` | Yes | None | Raw provider payload for the lap | `{"compound":"MEDIUM","deleted":false}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T19:42:00Z` |

### `RaceTelemetrySample`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the telemetry sample | `45001` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `driverNumber` | `Int` | Yes | Indexed with `sessionId` and `dateUtc` | Driver race number | `1` |
| `dateUtc` | `DateTime` | Yes | Indexed | Sample timestamp in UTC | `2026-05-03T19:41:12.450Z` |
| `speedKph` | `Int?` | No | None | Current speed in km/h | `312` |
| `rpm` | `Int?` | No | None | Engine RPM value | `11850` |
| `gear` | `Int?` | No | None | Current gear | `8` |
| `throttlePct` | `Float?` | No | None | Throttle percentage | `98.5` |
| `brakePct` | `Float?` | No | None | Brake percentage | `0.0` |
| `drsState` | `Int?` | No | None | Encoded DRS state | `1` |
| `raw` | `Json` | Yes | None | Raw telemetry payload | `{"source":"car_data"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T19:41:12.500Z` |

### `RaceLocationSample`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the location sample | `88012` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `driverNumber` | `Int` | Yes | Indexed with `sessionId` and `dateUtc` | Driver race number | `1` |
| `dateUtc` | `DateTime` | Yes | Indexed | Sample timestamp in UTC | `2026-05-03T19:41:12.450Z` |
| `x` | `Float?` | No | None | X coordinate on track map | `124.52` |
| `y` | `Float?` | No | None | Y coordinate on track map | `-52.11` |
| `z` | `Float?` | No | None | Z coordinate or elevation value | `0.73` |
| `raw` | `Json` | Yes | None | Raw provider location payload | `{"trackStatus":"green"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T19:41:12.500Z` |

### `RacePositionSample`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the position sample | `2200` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `driverNumber` | `Int` | Yes | Indexed with `sessionId` and `dateUtc` | Driver race number | `1` |
| `dateUtc` | `DateTime` | Yes | Indexed | Sample timestamp in UTC | `2026-05-03T19:41:13Z` |
| `lapNumber` | `Int?` | No | None | Lap number at sample time | `27` |
| `position` | `Int?` | No | Indexed with `sessionId` | Track position or race rank | `1` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"interval":"LEADER"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T19:41:13.010Z` |

### `RaceIntervalSample`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the interval sample | `3300` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `driverNumber` | `Int` | Yes | Indexed with `sessionId` and `dateUtc` | Driver race number | `11` |
| `dateUtc` | `DateTime` | Yes | Indexed | Sample timestamp in UTC | `2026-05-03T19:41:13Z` |
| `gapToLeader` | `String?` | No | `varchar(40)` | Gap from this driver to the race leader | `+4.231` |
| `intervalToFrontDriver` | `String?` | No | `varchar(40)` | Gap to the driver immediately ahead | `+0.842` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"status":"running"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T19:41:13.020Z` |

### `RaceStint`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the stint row | `90` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `driverNumber` | `Int` | Yes | Indexed with `sessionId` | Driver race number | `16` |
| `stintNumber` | `Int?` | No | None | Sequential stint number for the driver | `2` |
| `lapStart` | `Int?` | No | Indexed with `lapEnd` | First lap of the stint | `21` |
| `lapEnd` | `Int?` | No | Indexed with `lapStart` | Last lap of the stint | `39` |
| `compound` | `String?` | No | `varchar(40)` | Tyre compound used during the stint | `HARD` |
| `tyreAgeLapsStart` | `Int?` | No | None | Tyre age at the start of the stint | `3` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"newTyre":false}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T20:00:00Z` |

### `RacePitStop`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the pit stop row | `12` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `driverNumber` | `Int` | Yes | Indexed with `dateUtc` | Driver race number | `16` |
| `lapNumber` | `Int?` | No | Indexed with `sessionId` | Lap on which the pit stop occurred | `20` |
| `dateUtc` | `DateTime?` | No | Indexed with `driverNumber` | Timestamp of the stop | `2026-05-03T19:58:20Z` |
| `pitDurationSec` | `Float?` | No | None | Duration of the pit stop in seconds | `2.34` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"stopped":true}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T19:58:21Z` |

### `RaceControlEvent`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the race control event | `301` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `dateUtc` | `DateTime` | Yes | Indexed | Event timestamp in UTC | `2026-05-03T20:15:00Z` |
| `lapNumber` | `Int?` | No | None | Lap associated with the event | `41` |
| `driverNumber` | `Int?` | No | None | Driver concerned by the event when applicable | `4` |
| `category` | `String?` | No | `varchar(60)`, indexed with `sessionId` | Main category of the event | `incident` |
| `flag` | `String?` | No | `varchar(40)`, indexed with `sessionId` | Flag associated with the event | `yellow` |
| `scope` | `String?` | No | `varchar(80)` | Scope or area affected by the event | `sector-2` |
| `message` | `String?` | No | None | Human-readable race control message | `Car 4 noted for track limits` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"messageId":991}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T20:15:00.500Z` |

### `RaceWeatherSample`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the weather sample | `700` |
| `sessionId` | `String` | Yes | Indexed with `dateUtc` | Session identifier | `session_2026_miami_race` |
| `dateUtc` | `DateTime` | Yes | Indexed | Sample timestamp in UTC | `2026-05-03T19:41:00Z` |
| `airTempC` | `Float?` | No | None | Air temperature in Celsius | `28.4` |
| `trackTempC` | `Float?` | No | None | Track temperature in Celsius | `41.7` |
| `humidityPct` | `Float?` | No | None | Humidity percentage | `68.0` |
| `pressureHpa` | `Float?` | No | None | Atmospheric pressure in hPa | `1011.2` |
| `windSpeedKph` | `Float?` | No | None | Wind speed in km/h | `14.3` |
| `windDirectionDeg` | `Int?` | No | None | Wind direction in degrees | `220` |
| `rainfallMm` | `Float?` | No | None | Rainfall value in millimeters | `0.0` |
| `raw` | `Json` | Yes | None | Raw weather payload | `{"trackStatus":"dry"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T19:41:00.100Z` |

### `RaceHighlight`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the highlight | `1fd20367-3317-4e47-b644-283ff7872ec2` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `timestampMs` | `Int` | Yes | None | Highlight timestamp in milliseconds from session start | `5320000` |
| `type` | `String` | Yes | `varchar(50)` | Highlight category | `overtake` |
| `summary` | `String` | Yes | None | Human-readable summary of the highlight | `Driver 1 overtakes driver 16 for the lead` |
| `metadata` | `Json` | Yes | None | Extra structured metadata attached to the highlight | `{"drivers":[1,16],"corner":"Turn 11"}` |

### `OvertakeEvent`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the overtake event | `44` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `dateUtc` | `DateTime?` | No | Indexed with `sessionId` | Event timestamp in UTC | `2026-05-03T20:12:14Z` |
| `lapNumber` | `Int?` | No | Indexed with `sessionId` | Lap of the overtake | `39` |
| `overtakingDriverNumber` | `Int?` | No | None | Driver making the pass | `1` |
| `overtakenDriverNumber` | `Int?` | No | None | Driver being overtaken | `16` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"corner":"Turn 11"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T20:12:14.100Z` |

### `TeamRadioMessage`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the radio message | `78` |
| `sessionId` | `String` | Yes | Indexed | Session identifier | `session_2026_miami_race` |
| `driverNumber` | `Int` | Yes | Indexed with `sessionId` and `dateUtc` | Driver race number | `44` |
| `dateUtc` | `DateTime?` | No | Indexed | Radio timestamp in UTC | `2026-05-03T20:05:20Z` |
| `recordingUrl` | `String?` | No | `varchar(500)` | URL of the radio recording | `https://media.example.com/radio/44/clip-12.mp3` |
| `transcript` | `String?` | No | None | Optional text transcription of the message | `Tyres are gone, mate.` |
| `raw` | `Json` | Yes | None | Raw provider payload | `{"channel":"team_radio"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T20:05:21Z` |

### `SessionDriverBroadcast`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the broadcast link row | `5f836e95-6620-4dab-b913-3f1e15f31b84` |
| `sessionId` | `String` | Yes | Unique with `driverId`, indexed | Session identifier | `session_2026_miami_race` |
| `driverId` | `String` | Yes | Unique with `sessionId`, indexed | Internal driver identifier used by the app | `driver_1` |
| `broadcastUrl` | `String` | Yes | `varchar(255)` | Video or stream URL for the driver's onboard feed | `https://stream.example.com/onboard/1` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T18:50:00Z` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last update timestamp | `2026-05-03T18:55:00Z` |

### `RaceArchive`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the archive | `98c9f3fd-5305-4af5-b5c0-790c8fe3d4ff` |
| `sessionId` | `String` | Yes | Indexed with `generatedAt` | Session identifier covered by the archive | `session_2026_miami_race` |
| `mode` | `ArchiveMode` | Yes | Default `full` | Archive generation mode | `driver_focused` |
| `generatedAt` | `DateTime` | Yes | Default `now()` | Timestamp when the archive was generated | `2026-05-03T22:00:00Z` |
| `sourceMeetingKey` | `Int?` | No | Indexed with `sourceSessionKey` | Upstream meeting identifier | `1205` |
| `sourceSessionKey` | `Int?` | No | Indexed with `sourceMeetingKey` | Upstream session identifier | `8892` |
| `sourceDriverNumber` | `Int?` | No | None | Optional driver number for driver-focused archives | `1` |
| `metadata` | `Json` | Yes | None | Archive metadata and generation context | `{"producer":"ingestion-service","version":"1.0"}` |
| `counts` | `Json` | Yes | None | Aggregate counts per exported dataset | `{"laps":57,"telemetrySamples":45000}` |
| `fetchErrors` | `Json?` | No | None | Optional fetch or generation errors | `{"teamRadio":"timeout"}` |

### `RaceDatasetChunk`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `BigInt` | Yes | Primary key, auto-increment | Unique identifier of the chunk row | `1` |
| `archiveId` | `String` | Yes | Foreign key to `RaceArchive.id`, unique with `dataset` and `chunkIndex` | Parent archive identifier | `98c9f3fd-5305-4af5-b5c0-790c8fe3d4ff` |
| `dataset` | `String` | Yes | `varchar(50)`, indexed | Dataset name stored in the chunk | `telemetry` |
| `chunkIndex` | `Int` | Yes | Default `0`, unique with `archiveId` and `dataset` | Position of the chunk inside the dataset | `0` |
| `rowCount` | `Int` | Yes | Default `0` | Number of rows included in the chunk | `1000` |
| `rows` | `Json` | Yes | None | Chunk payload containing exported rows | `[{"driverNumber":1,"speedKph":312}]` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp | `2026-05-03T22:00:01Z` |

## Constraints and indexes

| Item | Type | Description |
| --- | --- | --- |
| `RaceLap(sessionId, driverNumber, lapNumber)` | Unique constraint | Prevents duplicate lap rows for the same driver and lap |
| `RaceDatasetChunk(archiveId, dataset, chunkIndex)` | Unique constraint | Prevents duplicate chunk positions inside an archive dataset |
| `RaceArchive.id -> RaceDatasetChunk.archiveId` | Foreign key | Enforces the archive-to-chunk relationship |
| `(sessionId, dateUtc)` indexes | Composite indexes | Support timeline queries over time-series tables |
| `(sessionId, driverNumber, dateUtc)` indexes | Composite indexes | Support driver-scoped time-series queries |
| `(sessionId, lapNumber)` and `(sessionId, position)` indexes | Composite indexes | Support race flow and ranking lookups |
| `(sessionId, generatedAt desc)` | Composite index | Supports latest archive retrieval per session |
