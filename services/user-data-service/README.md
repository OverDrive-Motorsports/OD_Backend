# user-data-service

`user-data-service` is the future service responsible for user-centric data such as profile information, preferences, and saved settings.

For now, it only exposes a healthcheck endpoint so we can validate the service bootstrapping and keep the architecture stable while the real business logic is still being built.

## Run

```bash
go run ./services/user-data-service
```

Default port: `3002`

## Current behavior

- exposes `GET /health`
- returns the service status and service name

## Planned scope

- user profiles
- preferences and personalization
- saved layouts
- account settings
