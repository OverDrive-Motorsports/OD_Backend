# ingestion-service

`ingestion-service` is the future service responsible for collecting and transforming data coming from external providers.

For now, it only exposes a healthcheck endpoint so we can validate the service bootstrapping and keep the architecture stable while the real ingestion logic is still being built.

## Run

```bash
go run ./services/ingestion-service
```

Default port: `3005`

## Current behavior

- exposes `GET /health`
- returns the service status and service name

## Planned scope

- external API collection
- payload transformation
- ingestion workflows
- provider integrations
