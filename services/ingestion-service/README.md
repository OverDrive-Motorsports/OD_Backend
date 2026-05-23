# ingestion-service

`ingestion-service` collects provider data, maps it into the internal OverDrive ingestion schema, and dispatches normalized batches to downstream services.

The first provider implemented here is `OpenF1`.

## Run

```bash
go run ./services/ingestion-service
```

Default port: `3005`

## Current behavior

- exposes `GET /health`
- exposes `POST /providers/openf1/ingestions`
- fetches supported OpenF1 resources with retry and timeout handling
- maps provider rows into an internal normalized batch format
- dispatches catalog datasets to `championship-service`
- dispatches timing and race datasets to `race-data-service`

## Supported OpenF1 resources

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

## Operational notes

- Ingest `meetings`, `sessions`, and `drivers` before standings or race-data resources.
- `starting_grid` falls back to the relevant qualifying session when OpenF1 does not expose grid data directly on the Race session.
- `car_data`, `location`, and `intervals` can be large. Use `driver_number` where possible and keep `DISPATCH_TIMEOUT` high enough for downstream Prisma writes.

## Example request

```bash
curl -X POST http://localhost:3005/providers/openf1/ingestions \
  -H 'Content-Type: application/json' \
  -d '{
    "meeting_key": 1255,
    "session_key": 9998,
    "driver_number": 63,
    "resources": ["drivers", "laps", "car_data", "position"],
    "dispatch": true
  }'
```
