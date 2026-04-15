# race-data-service

`race-data-service` is the future service responsible for race-oriented read models and normalized race data.

For now, it only exposes a healthcheck endpoint so we can validate the service bootstrapping and keep the architecture stable while the real race data logic is still being built.

## Run

```bash
go run ./services/race-data-service
```

Default port: `3004`

Database ownership: `race-data-service` owns `RACE_DATA_DATABASE_URL`, which should target its dedicated `overdrive_race_data` database locally.

## Current behavior

- exposes `GET /health`
- returns the service status and service name

## Planned scope

- session read models
- normalized race datasets
- timing and telemetry access
- derived race views
