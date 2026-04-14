# auth-service

`auth-service` is the future authentication and authorization service of the platform.

For now, it only exposes a healthcheck endpoint so we can validate the service bootstrapping and keep the architecture stable while the real auth logic is still being built.

## Run

```bash
go run ./services/auth-service
```

Default port: `3001`

Database ownership: `auth-service` owns `AUTH_DATABASE_URL`, which should target its dedicated `overdrive_auth` database locally.

## Current behavior

- exposes `GET /health`
- returns the service status and service name

## Planned scope

- login and identity flows
- access and refresh tokens
- authorization rules
- account security policies
