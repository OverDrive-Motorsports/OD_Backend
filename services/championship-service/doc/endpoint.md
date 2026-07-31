# championship-service endpoints

## `GET /health`

Returns the runtime status of the service.

## `POST /internal/ingestion/batches`

Internal endpoint used by `ingestion-service` to push normalized catalog and standings datasets.

Accepted dataset names:

- `event_catalog`
- `session_catalog`
- `driver_catalog`
- `session_result`
- `starting_grid`
- `driver_championship_standings`
- `team_championship_standings`

## Public catalog endpoints

All responses are **camelCase** JSON. List endpoints return **bare JSON arrays**
(`[...]`), never a `{ "count", "data" }` envelope.

- `GET /championships`
- `GET /championships/{code}/events` (query: `season`)
- `GET /events/{eventId}`
- `GET /events/{eventId}/sessions` (query: `type`)
- `GET /sessions/{sessionId}`
- `GET /sessions/{sessionId}/drivers` (query: `teamId`)
- `GET /sessions/{sessionId}/teams`
- `GET /sessions/{sessionId}/datasets/{dataset}`
- `GET /sessions/{sessionId}/standings` (query: `driverNumber`) — generic standings (currently backed by session race results)
- `GET /sessions/{sessionId}/standings/race` — deprecated alias of `/standings`, kept for backward compatibility with existing internal consumers
- `GET /sessions/{sessionId}/broadcast`
- `GET /drivers/{driverNumber}/profile` (query: `championshipCode`) — global, session-independent driver profile

Known gap: `GET /sessions/{sessionId}` does not populate `weatherAtStart` — this
service has no weather data source (weather samples are owned by
`race-data-service`). The field is simply omitted from the response today; wiring
cross-service enrichment is left for a follow-up.

Known gap: `driverPicture` on `GET /drivers/{driverNumber}/profile` is always
empty — no data source is currently ingested for driver headshots.

Supported session datasets:

- `session_result`
- `starting_grid`
- `championship_drivers`
- `championship_teams`

`starting_grid` may be sourced from the relevant qualifying session when OpenF1 does not expose grid data directly on the Race session. It is still persisted against the requested Race session ID.

### Example

```bash
curl http://localhost:3003/championships
curl http://localhost:3003/championships/f1/events
curl http://localhost:3003/events/<EVENT_ID>/sessions
curl http://localhost:3003/sessions/<SESSION_ID>/datasets/session_result
curl http://localhost:3003/sessions/<SESSION_ID>/datasets/starting_grid
```
