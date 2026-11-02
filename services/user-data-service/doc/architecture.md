# user-data-service architecture

## Service purpose

`user-data-service` stores the personalised user experience: today, the AR/VR layout presets of each user (`/presets`). `ProviderConnection` exists in the schema but has no endpoint yet.

## Current architecture

This service follows a simple layered architecture with clear responsibilities:

- `main.go`: bootstraps the service configuration, creates the use case and controller, and starts the HTTP server.
- `src/adapters/http`: contains the HTTP controller and route registration. This is the entry point for incoming HTTP requests.
- `src/core/usecases`: contains application logic. A use case receives data from the controller and returns a domain result.
- `src/core/ports`: contains interfaces used to decouple the controller layer from the application logic.
- `src/core/domain`: contains business entities and response models used by the service.
- `src/adapters/repository/prisma`: Prisma-backed storage (`PresetRepository`).
- `shared/bootstrap`: shared module used to load environment variables and common runtime configuration.
- `shared/authjwt`: shared verifier of the access tokens issued by `auth-service` (also used by `auth-service` and the gateway).

## Authentication

Presets are private per-user data. `src/adapters/http/middleware.go` (`requireUser`) verifies
the `Authorization: Bearer <access token>` header with `shared/authjwt` (HS256 only, expiry
required, secret `AUTH_JWT_SECRET`) and puts the token's `sub` in the request context. It is
the **only** source of the user ID; every repository query is scoped by it, so another user's
preset behaves like a missing one (`404`). The service refuses to start without
`AUTH_JWT_SECRET`. The gateway checks the same token on `/presets` before proxying (defence in
depth), but the service never trusts the gateway alone since its port is also reachable
directly in local Docker.

## Presets

A preset is `{ name, isDefault, data }`, where `data` is the client's layout document stored
as-is (a JSON object, at most 256 KiB). Setting a default is done in one Prisma transaction
(clear the user's other defaults + update/create), so a user never has two defaults,
Prisma cannot express a partial unique index (`UNIQUE (userId) WHERE isDefault`).

## Request flow

For `GET /health`, the request flow is:

1. `main.go` loads the config and wires the dependencies.
2. `src/adapters/http/health.routes.go` registers `GET /health`.
3. `src/adapters/http/health.controller.go` receives the HTTP request.
4. The controller calls the use case through the `ports.HealthUseCase` interface.
5. `src/core/usecases/get_health.usecase.go` builds the `domain.HealthStatus` response.
6. The controller serializes the response as JSON.

## How to add a new endpoint

Use this flow when adding a new endpoint:

1. Create or update the domain model in `src/core/domain` if the endpoint needs a new response or business entity.
2. Define or extend the interface in `src/core/ports` for the new use case contract.
3. Implement the business logic in `src/core/usecases`.
4. Add a controller method in `src/adapters/http` to parse the request and call the use case.
5. Register the route in a route file inside `src/adapters/http`.
6. Wire the new use case and controller dependency in `main.go`.
7. Document the endpoint in `doc/endpoint.md`.

## Practical guideline

Keep HTTP concerns in the adapter layer, business rules in use cases, and data structures in the domain layer. This keeps the service easy to extend and avoids mixing transport logic with business logic.
