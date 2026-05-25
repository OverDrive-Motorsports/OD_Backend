# ingestion-service endpoints

## `GET /health`

Returns the runtime status of the service.

## `POST /providers/openf1/ingestions`

Triggers a manual OpenF1 ingestion batch.

### Request body

```json
{
  "meeting_key": 1255,
  "session_key": 9998,
  "driver_number": 63,
  "resources": ["drivers", "laps", "car_data", "position"],
  "dispatch": true
}
```

### Fields

| Field | Type | Mandatory | Description |
| --- | --- | --- | --- |
| `meeting_key` | `int` | Yes | OpenF1 meeting identifier |
| `session_key` | `int` | Yes | OpenF1 session identifier |
| `driver_number` | `int` | No | Restricts driver-scoped datasets to one driver |
| `resources` | `string[]` | No | Explicit list of OpenF1 resources to fetch. If omitted, all supported resources are fetched |
| `dispatch` | `bool` | No | If `false`, fetch and mapping still run but no downstream dispatch is performed |

### Supported OpenF1 resources

- `meetings`
- `sessions`
- `drivers`
- `championship_drivers`
- `championship_teams`
- `session_result`
- `starting_grid`
- `laps`
- `car_data`
- `location`
- `position`
- `intervals`
- `stints`
- `pit`
- `weather`
- `team_radio`
- `overtakes`
- `race_control`

### Persistence mapping

| OpenF1 resource | Target service | Main Prisma table(s) |
| --- | --- | --- |
| `meetings` | `championship-service` | `Event` |
| `sessions` | `championship-service` | `Session` |
| `drivers` | `championship-service` | `Driver`, `Team` |
| `session_result` | `championship-service` | `SessionResultRow` |
| `starting_grid` | `championship-service` | `StartingGridRow` |
| `championship_drivers` | `championship-service` | `DriverChampionshipStanding` |
| `championship_teams` | `championship-service` | `TeamChampionshipStanding` |
| `laps` | `race-data-service` | `RaceLap`, `SessionDriverBroadcast` |
| `car_data` | `race-data-service` | `RaceTelemetrySample`, `SessionDriverBroadcast` |
| `location` | `race-data-service` | `RaceLocationSample`, `SessionDriverBroadcast` |
| `position` | `race-data-service` | `RacePositionSample`, `SessionDriverBroadcast` |
| `intervals` | `race-data-service` | `RaceIntervalSample`, `SessionDriverBroadcast` |
| `stints` | `race-data-service` | `RaceStint`, `SessionDriverBroadcast` |
| `pit` | `race-data-service` | `RacePitStop`, `SessionDriverBroadcast` |
| `weather` | `race-data-service` | `RaceWeatherSample` |
| `team_radio` | `race-data-service` | `TeamRadioMessage`, `SessionDriverBroadcast` |
| `overtakes` | `race-data-service` | `OvertakeEvent` |
| `race_control` | `race-data-service` | `RaceControlEvent` |

### Recommended ingestion order

Ingest catalog data before standings or race-data datasets:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["meetings","sessions","drivers","session_result"],"dispatch":true}'
```

Then add championship standings and grid data:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["championship_drivers","championship_teams","starting_grid"],"dispatch":true}'
```

Then add race-data datasets. Large resources should be sent in smaller batches:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"resources":["position","intervals","stints","pit","weather","overtakes","race_control"],"dispatch":true}'
```

Driver-scoped telemetry and location payloads can be large. Prefer adding `driver_number`:

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{"meeting_key":1255,"session_key":9998,"driver_number":63,"resources":["car_data","location","team_radio"],"dispatch":true}'
```

### Starting grid fallback

OpenF1 does not always expose `starting_grid` directly on Race sessions. When `starting_grid` returns no rows for a Race session, the provider adapter looks for the relevant qualifying session from the same meeting and persists that grid against the requested Race session.

For sprint sessions, the adapter first looks for the preceding Sprint Qualifying session. For normal Race sessions, it looks for the preceding non-sprint Qualifying session.

### Returned message

Status: `202 Accepted`

```json
{
  "provider": "openf1",
  "batch_id": "openf1-1770000000000",
  "meeting_key": 1255,
  "session_key": 9998,
  "resources": [
    {
      "resource": "drivers",
      "dataset_name": "driver_catalog",
      "target_service": "championship-service",
      "row_count": 20,
      "dispatched": true
    }
  ],
  "dispatches": [
    {
      "service": "championship-service",
      "dataset_count": 1,
      "row_count": 20
    }
  ],
  "dispatch_enabled": true,
  "triggered_at_utc": "2026-04-24T10:00:00Z",
  "completed_at_utc": "2026-04-24T10:00:02Z"
}
```
