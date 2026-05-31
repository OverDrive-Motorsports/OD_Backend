# How To Contribute (Gateway)

This document explains how to contribute to the gateway while keeping the architecture clean and easy to maintain.

## Architecture Rule

Follow this direction of dependencies only:

1. `core` (domain, ports, use cases)
2. `adapters` (inbound/outbound implementations)
3. `app` (composition/wiring)
4. `main.go` (process bootstrap)

`core` must never import `adapters`.

## Folder Responsibilities

- `internal/core/domain`: business models used by use cases
- `internal/core/ports`: interfaces for external dependencies
- `internal/core/usecases`: pure application logic
- `internal/adapters/outbound`: concrete implementations (auth provider, etc.)
- `internal/adapters/inbound/http`: HTTP handlers, middleware, proxy behavior
- `internal/app`: dependency injection and runtime assembly
- `internal/config`: env parsing and route registry

## Add a New Proxied Route

1. Open or create the relevant route registry file in `internal/config/`:
   - `routes_auth.go`
   - `routes_user_data.go`
   - `routes_championship.go`
   - `routes_race_data.go`
   - `routes_ingestion.go`
2. Add the new prefix to the `proxied` map of that service.
3. If needed, add/update service health path in the `health` map.
4. If the service is new, wire it in `routes_registry.go` and `config.go`.
5. Add corresponding env variable in `gateway/.env.template` and `gateway/.env`.
6. Document changes in `gateway/README.md` and `gateway/ROUTES.md`.
7. Test with `curl` through the gateway.

## Replace Static Auth With JWT

1. Keep `core/ports/token_validator.go` unchanged if possible.
2. Add a new adapter in `internal/adapters/outbound/auth` (for example `jwt_token_validator.go`).
3. Update `internal/app/bootstrap.go` to inject the new validator.
4. Keep HTTP middleware unchanged (it already depends on use case callback).

## Add a New Use Case

1. Create or extend domain model in `core/domain` if needed.
2. Add required interface(s) in `core/ports`.
3. Implement use case in `core/usecases`.
4. Implement adapter(s) for new ports in `adapters/outbound`.
5. Wire everything in `internal/app/bootstrap.go`.

## Coding Guidelines

- Keep use cases deterministic and framework-agnostic.
- Keep adapters focused on I/O and translation.
- Do not put business rules in middleware/proxy code.
- Keep HTTP responses in consistent JSON shape.
- Prefer small files with explicit naming.

## Smoke Test Checklist

```bash
# Read configured token from gateway/.env
GATEWAY_TOKEN="$(grep '^GATEWAY_AUTH_TOKEN=' gateway/.env | cut -d= -f2-)"

# Gateway health (no auth header)
curl -i http://localhost:3000/health

# Gateway health with valid auth header
curl -i -H "Authorization: Bearer ${GATEWAY_TOKEN}" http://localhost:3000/health

# Invalid provided token must fail
curl -i -H "Authorization: Bearer invalid" http://localhost:3000/health

# Service health via gateway
curl -i http://localhost:3000/health/auth
curl -i http://localhost:3000/health/user-data
curl -i http://localhost:3000/health/championship
curl -i http://localhost:3000/health/race-data
curl -i http://localhost:3000/health/ingestion
```

## Definition of Done

- Route/use case works end-to-end
- `README.md` and `ROUTES.md` updated
- No architectural boundary violation (`core` importing `adapters`)
- `go test ./gateway/...` passes
