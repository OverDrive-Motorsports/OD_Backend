# OverDrive API Endpoints

Base URL: `http://localhost:8080`

## Notes

- `GET /getrace` or `GET /api/v1/race/getrace` must be called first to fetch and store data.
- Read endpoints use the merged latest session view, so several driver imports are aggregated together.
- Historical navigation is now supported through explicit session-scoped endpoints.
- Recommended navigation flow: `championship -> race -> session -> session-scoped data endpoints`.
- Legacy routes are still available for `/getrace` and `/sendrace`.

## Health

- `GET /`
  - Returns service info and route list.
- `GET /health`
  - Returns liveness status.

## Catalog

- `GET /api/v1/championships`
  - Returns all stored championships.

- `GET /api/v1/championships/{code}/races`
  - Returns the races stored for one championship code such as `f1`.

- `GET /api/v1/races/{raceId}`
  - Returns one stored race entry.

- `GET /api/v1/races/{raceId}/sessions`
  - Returns stored sessions attached to one race.

- `GET /api/v1/sessions/{sessionId}`
  - Returns one stored session entry, including latest stored dataset counts when available.

## Session-Scoped Race Data

- `GET /api/v1/sessions/{sessionId}/archive`
  - Returns the merged stored archive for one explicit session.

- `GET /api/v1/sessions/{sessionId}/metadata`
  - Returns meeting metadata, race session metadata, all sessions, and stored timestamp for one explicit session.

- `GET /api/v1/sessions/{sessionId}/datasets`
  - Returns the dataset catalog for one explicit session.

- `GET /api/v1/sessions/{sessionId}/datasets/{dataset}`
  - Returns one merged dataset for one explicit session.
  - Supported dataset values:
    - `drivers`
    - `laps`
    - `car_data`
    - `telemetry`
    - `location`
    - `position`
    - `intervals`
    - `stints`
    - `pit`
    - `team_radio`
    - `radio`
    - `race_control`
    - `facts`
    - `weather`
    - `session_result`
    - `result`
    - `starting_grid`
    - `grid`
    - `overtakes`
    - `championship_drivers`
    - `championship_teams`
    - `constructors`

- `GET /api/v1/sessions/{sessionId}/drivers`
  - Returns the drivers available in one explicit session.

- `GET /api/v1/sessions/{sessionId}/teams`
  - Returns teams inferred from the session driver roster.

- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}`
  - Returns the full race payload for one driver in one explicit session.

- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/profile`
  - Returns the driver profile only for one explicit session.

- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/broadcast`
  - Returns the driver-specific broadcast URL stored for one explicit session.

- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/laps`
- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/telemetry`
- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/location`
- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/position`
- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/intervals`
- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/stints`
- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/pit`
- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/radio`
- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/result`
  - Return one driver-scoped dataset for one driver in one explicit session.

- `GET /api/v1/sessions/{sessionId}/weather`
  - Returns weather samples for one explicit session.

- `GET /api/v1/sessions/{sessionId}/facts`
  - Returns race control events for one explicit session.

- `GET /api/v1/sessions/{sessionId}/standings/race`
  - Returns the in-race standings snapshot for one explicit session.
  - Optional query param:
    - `at` in RFC3339 format

- `GET /api/v1/sessions/{sessionId}/broadcast`
  - Returns the stored session broadcast URL.

## Fetch And Storage

- `GET /getrace`
- `GET /api/v1/race/getrace`
  - Query params:
    - `year`
    - `country`
    - `meeting`
    - `driver_number`
  - Fetches OpenF1 data and stores it in PostgreSQL.

- `GET /api/v1/race/cache`
- `GET /api/v1/race/storage`
  - Returns storage status for the latest session import.

## Full Race Payload

- `GET /sendrace`
- `POST /sendrace`
- `GET /api/v1/race/sendrace`
- `POST /api/v1/race/sendrace`
  - Returns the merged latest race payload.

- `GET /api/v1/race/metadata`
  - Returns race metadata, meeting info, race session info, and all session references.

## Dataset Catalog

- `GET /api/v1/race/datasets`
  - Returns available datasets with row counts.

- `GET /api/v1/race/datasets/{dataset}`
  - Returns one merged dataset by name.
  - Supported dataset values:
    - `drivers`
    - `laps`
    - `car_data`
    - `telemetry`
    - `location`
    - `position`
    - `intervals`
    - `stints`
    - `pit`
    - `team_radio`
    - `radio`
    - `race_control`
    - `facts`
    - `weather`
    - `session_result`
    - `result`
    - `starting_grid`
    - `grid`
    - `overtakes`
    - `championship_drivers`
    - `championship_teams`
    - `constructors`

## Drivers

- `GET /api/v1/race/drivers`
  - Returns the merged list of drivers available in the active race.

- `GET /api/v1/race/drivers/{driverNumber}`
  - Returns the full race payload for one driver.
  - Includes profile, counts, and all driver-scoped datasets.

- `GET /api/v1/race/drivers/{driverNumber}/profile`
  - Returns the driver profile only.

- `GET /api/v1/race/drivers/{driverNumber}/laps`
- `GET /api/v1/race/drivers/{driverNumber}/telemetry`
- `GET /api/v1/race/drivers/{driverNumber}/location`
- `GET /api/v1/race/drivers/{driverNumber}/position`
- `GET /api/v1/race/drivers/{driverNumber}/intervals`
- `GET /api/v1/race/drivers/{driverNumber}/stints`
- `GET /api/v1/race/drivers/{driverNumber}/pit`
- `GET /api/v1/race/drivers/{driverNumber}/radio`
- `GET /api/v1/race/drivers/{driverNumber}/result`
  - Return one driver-scoped dataset for one driver.

- `GET /api/v1/race/driver?driver_number={driverNumber}`
  - Legacy endpoint returning the full race payload for one driver.

## Teams

- `GET /api/v1/race/teams`
  - Returns teams inferred from the merged drivers dataset.

## Championship

- `GET /api/v1/race/championship/drivers`
  - Returns driver championship standings.

- `GET /api/v1/race/championship/constructors`
  - Returns constructor championship standings.

## Race Control And Weather

- `GET /api/v1/race/weather`
  - Returns race weather samples.

- `GET /api/v1/race/facts`
  - Returns race control events such as flags and incidents.

## Standings And Broadcast

- `GET /api/v1/race/standings/race`
  - Returns the latest in-race standings snapshot from the merged `position` dataset.
  - Optional query param:
    - `at` in RFC3339 format, for example `2025-03-16T04:45:00Z`

- `GET /api/v1/race/video-url`
  - Returns the placeholder broadcast URL.

## Quick Examples

```bash
curl "http://localhost:8080/api/v1/championships"
curl "http://localhost:8080/api/v1/championships/f1/races"
curl "http://localhost:8080/api/v1/race/getrace?year=2025&country=Australia&meeting=Australian%20Grand%20Prix&driver_number=0"
curl "http://localhost:8080/api/v1/races/race-aus-2025/sessions"
curl "http://localhost:8080/api/v1/sessions/session-race-9693/archive"
curl "http://localhost:8080/api/v1/sessions/session-race-9693/drivers"
curl "http://localhost:8080/api/v1/sessions/session-race-9693/drivers/63/broadcast"
curl "http://localhost:8080/api/v1/race/drivers"
curl "http://localhost:8080/api/v1/race/drivers/63/profile"
curl "http://localhost:8080/api/v1/race/drivers/63/telemetry"
curl "http://localhost:8080/api/v1/race/datasets/weather"
curl "http://localhost:8080/api/v1/race/standings/race?at=2025-03-16T04:45:00Z"
```
