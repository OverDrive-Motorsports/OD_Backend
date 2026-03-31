# OverDrive API Endpoints

Base URL: `http://localhost:8080`

## Notes

- `GET /getrace` or `GET /api/v1/race/getrace` must be called first to fetch and store data.
- Session-scoped endpoints are the preferred read API because they target one explicit stored session.
- `/api/v1/race/*` endpoints are legacy convenience routes that resolve against the merged latest stored session.
- Latest-session reads use the merged latest session view, so several driver imports are aggregated together.
- Recommended navigation flow: `provider -> championship -> event -> session -> session-scoped data endpoints`.
- Legacy routes are still available for `/getrace` and `/sendrace`.
- Dataset rows that expose `driver_number` are enriched with `driver_name` and `team_name` when the corresponding driver exists in the stored session.
- There is currently no dedicated `GET /api/v1/championships/{code}/drivers` or `GET /api/v1/championships/{code}/teams` endpoint.

## Health

- `GET /`
  - Returns service info and route list.
- `GET /health`
  - Returns liveness status.

## Catalog

- `GET /api/v1/championships`
  - Returns all stored championships.

- `GET /api/v1/championships/{code}/events`
  - Returns the events stored for one championship code such as `f1`.
  - The hierarchy is strict: events belong to championships, sessions belong to events.

- `GET /api/v1/events/{eventId}`
  - Returns one stored event entry.

- `GET /api/v1/events/{eventId}/sessions`
  - Returns stored sessions attached to one event.

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
  - Rows containing `driver_number` are enriched with:
    - `driver_name`
    - `team_name`
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
  - Returns the full session payload for one driver in one explicit session.

- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/profile`
  - Returns the driver profile only for one explicit session.

- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/broadcast`
  - Returns the driver-specific broadcast URL stored for one explicit session.

- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/{resource}`
  - Return one driver-scoped dataset for one driver in one explicit session.
  - Supported resource values:
    - `laps`
    - `telemetry`
    - `location`
    - `position`
    - `intervals`
    - `stints`
    - `pit`
    - `radio`
    - `result`

- `GET /api/v1/sessions/{sessionId}/drivers/{driverNumber}/laps/{lapNumber}/location`
  - Returns all stored `x/y/z` location samples for one driver during one lap.
  - The backend resolves the lap time window from:
    - the lap `date_start`
    - the next lap `date_start`
    - or `lap_duration` as fallback when the next lap is unavailable

- `GET /api/v1/sessions/{sessionId}/weather`
  - Returns weather samples for one explicit session.

- `GET /api/v1/sessions/{sessionId}/facts`
  - Returns race control events for one explicit session.

- `GET /api/v1/sessions/{sessionId}/standings/race`
  - Returns the in-session standings snapshot for one explicit session.
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

## Latest-Session Convenience Endpoints

- `GET /sendrace`
- `POST /sendrace`
- `GET /api/v1/race/sendrace`
- `POST /api/v1/race/sendrace`
  - Returns the merged latest session payload.

- `GET /api/v1/race/metadata`
  - Returns latest-session metadata, meeting info, race session info, and all session references.

## Dataset Catalog

- `GET /api/v1/race/datasets`
  - Returns available datasets with row counts.

- `GET /api/v1/race/datasets/{dataset}`
  - Returns one merged dataset by name.
  - Rows containing `driver_number` are enriched with:
    - `driver_name`
    - `team_name`
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

## Latest-Session Driver And Team Endpoints

- `GET /api/v1/race/drivers`
  - Returns the merged list of drivers available in the latest stored session.

- `GET /api/v1/race/teams`
  - Returns teams inferred from the merged drivers dataset.

- `GET /api/v1/race/drivers/{driverNumber}`
  - Returns the full latest-session payload for one driver.

- `GET /api/v1/race/drivers/{driverNumber}/profile`
  - Returns the driver profile only for the merged latest stored session.

- `GET /api/v1/race/drivers/{driverNumber}/{resource}`
  - Latest-session mirror of the session-scoped driver dataset endpoint.
  - It resolves data against the merged latest stored session instead of an explicit `sessionId`.
  - Supported resource values:
    - `laps`
    - `telemetry`
    - `location`
    - `position`
    - `intervals`
    - `stints`
    - `pit`
    - `radio`
    - `result`

- `GET /api/v1/race/drivers/{driverNumber}/laps/{lapNumber}/location`
  - Latest-session mirror of the lap-scoped location endpoint.
  - Returns all stored `x/y/z` samples for one driver during one lap in the merged latest stored session.

- `GET /api/v1/race/driver?driver_number={driverNumber}`
  - Legacy query-param variant returning the full latest-session payload for one driver.

## Championship Participants

- There is no dedicated `GET /api/v1/championships/{code}/drivers` endpoint yet.
- There is no dedicated `GET /api/v1/championships/{code}/teams` endpoint yet.
- Current workaround:
  - `GET /api/v1/championships/{code}/events`
  - `GET /api/v1/events/{eventId}/sessions`
  - `GET /api/v1/sessions/{sessionId}/drivers`
  - `GET /api/v1/sessions/{sessionId}/teams`

## Championship

- `GET /api/v1/race/championship/drivers`
  - Returns driver championship standings.

- `GET /api/v1/race/championship/constructors`
  - Returns constructor championship standings.

## Race Control And Weather

- `GET /api/v1/race/weather`
  - Returns latest-session weather samples.

- `GET /api/v1/race/facts`
  - Returns latest-session race control events such as flags and incidents.

## Standings And Broadcast

- `GET /api/v1/race/standings/race`
  - Returns the latest in-session standings snapshot from the merged `position` dataset.
  - Optional query param:
    - `at` in RFC3339 format, for example `2025-03-16T04:45:00Z`

- `GET /api/v1/race/video-url`
  - Returns the placeholder broadcast URL.

## Quick Examples

```bash
curl "http://localhost:8080/api/v1/championships"
curl "http://localhost:8080/api/v1/championships/f1/events"
curl "http://localhost:8080/api/v1/race/getrace?year=2025&country=Australia&meeting=Australian%20Grand%20Prix&driver_number=0"
curl "http://localhost:8080/api/v1/events/<EVENT_ID>/sessions"
curl "http://localhost:8080/api/v1/sessions/<SESSION_ID>/archive"
curl "http://localhost:8080/api/v1/sessions/<SESSION_ID>/drivers"
curl "http://localhost:8080/api/v1/sessions/<SESSION_ID>/datasets/championship_drivers"
curl "http://localhost:8080/api/v1/sessions/<SESSION_ID>/datasets/session_result"
curl "http://localhost:8080/api/v1/sessions/<SESSION_ID>/drivers/63/laps/27/location"
curl "http://localhost:8080/api/v1/sessions/<SESSION_ID>/drivers/63/broadcast"
curl "http://localhost:8080/api/v1/race/sendrace"
curl "http://localhost:8080/api/v1/race/datasets/weather"
curl "http://localhost:8080/api/v1/race/drivers/63/laps/27/location"
curl "http://localhost:8080/api/v1/race/standings/race?at=2025-03-16T04:45:00Z"
```
