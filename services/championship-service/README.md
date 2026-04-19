# championship-service

`championship-service` is the future service responsible for championship structure, seasons, events, and calendars.

For now, it only exposes a healthcheck endpoint so we can validate the service bootstrapping and keep the architecture stable while the real championship logic is still being built.

## Run

```bash
go run ./services/championship-service
```

Default port: `3003`

Database ownership: `championship-service` owns `CHAMPIONSHIP_DATABASE_URL`, which should target its dedicated `overdrive_championship` database locally.

## Current behavior

- exposes `GET /health`
- returns the service status and service name

## Planned scope

- championships and categories
- seasons and calendars
- event metadata
- competition structure
