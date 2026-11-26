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

## Live jobs

`POST /providers/openf1/live/{sessionKey}/start` starts a background job
(`src/core/usecases/live_job.go`, one per session, in memory):

1. It waits for 30 minutes before the session start (reading the dates again every 15 minutes),
   then ingests the catalog (`meetings`, `sessions`, `drivers`) like a manual ingestion, until the
   drivers are published, and again when the session starts and ends so its status follows.
2. Each round polls the resources that are due (`domain.LiveResources`) from the latest known row
   minus an overlap, up to 5 minutes at a time for `car_data` and `location` (which start one
   hour before the session, to include the laps to the grid). Rows identical to
   the previous fetch are not sent again.
3. New rows are mapped as usual and dispatched with `write_mode: merge`. The cursor of a resource
   only moves once its rows are acknowledged, so a failure is fetched again.
4. The job ends at the session end plus the grace period (after a last whole fetch of `laps`,
   `pit` and `stints`), on stop, with the service, or after too many failed rounds in a row once the
   session and its grace period are over or when OpenF1 refuses the credentials.

`SessionGuard` keeps a manual ingestion and a live job from writing the same session at once. The
guard and the jobs belong to one process: run a single instance of this service.

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
