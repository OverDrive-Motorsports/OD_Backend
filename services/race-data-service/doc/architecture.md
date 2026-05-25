# race-data-service architecture

## Service purpose

`race-data-service` exposes race data queries and normalized read models populated by ingestion batches.

## Current architecture

This service follows a simple layered architecture with clear responsibilities:

- `main.go`: bootstraps the service configuration, creates the use case and controller, and starts the HTTP server.
- `src/adapters/http`: contains the HTTP controller and route registration. This is the entry point for incoming HTTP requests.
- `src/core/usecases`: contains application logic. A use case receives data from the controller and returns a domain result.
- `src/core/ports`: contains interfaces used to decouple the controller layer from the application logic.
- `src/core/domain`: contains business entities and response models used by the service.
- `shared/bootstrap`: shared module used to load environment variables and common runtime configuration.

## Request flow

For a typical race-data endpoint, the request flow is:

1. `main.go` loads the config and wires the dependencies.
2. `src/adapters/http/ingestion.routes.go` creates the router and registers health, ingestion, and session routes.
3. The matching HTTP controller receives the request and parses path or body data.
4. The controller calls the use case through a port interface.
5. `src/core/usecases` builds the response from local race data and championship-service references.
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
