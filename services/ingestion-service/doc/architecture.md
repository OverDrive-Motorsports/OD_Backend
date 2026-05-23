# ingestion-service architecture

## Service purpose

`ingestion-service` is responsible for three things:

1. fetching provider data
2. mapping provider payloads into the internal OverDrive ingestion schema
3. dispatching normalized batches to downstream services

The first provider implemented is `OpenF1`.

## Current architecture

This service follows the project hexagonal structure:

- `main.go`: wires config, provider adapter, mapper, dispatcher, and HTTP adapters.
- `src/config`: service-specific runtime configuration.
- `src/adapters/http`: HTTP controllers and route registration.
- `src/adapters/providers/openf1`: OpenF1 client and mapping logic.
- `src/adapters/dispatch/http`: reusable HTTP dispatcher for downstream services.
- `src/core/usecases`: ingestion orchestration.
- `src/core/ports`: provider, mapper, and dispatcher contracts.
- `src/core/domain`: ingestion request and result models.
- `shared/contracts/ingestion`: normalized batch contract shared with downstream services.

## Request flow

1. `POST /providers/openf1/ingestions` receives an ingestion request.
2. The use case validates the request and decides which resources must be fetched.
3. The OpenF1 adapter fetches each resource with timeout and retry handling.
4. The mapper converts provider rows into normalized internal datasets.
5. Datasets are grouped by target service.
6. The dispatcher forwards each batch to the corresponding downstream service.
7. The controller returns a batch summary with fetched resources and dispatch results.

## Dispatch strategy

- `championship-service`
  - `meetings`
  - `sessions`
  - `drivers`
- `race-data-service`
  - `laps`
  - `car_data`
  - `location`
  - `position`
  - `intervals`
  - `stints`
  - `pit`
  - `weather`
  - `team_radio`

This routing is intentionally table-driven in the mapper so new providers or new datasets can be added without rewriting the use case.
