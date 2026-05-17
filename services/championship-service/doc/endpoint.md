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

- `GET /championships`
- `GET /championships/{code}/events`
- `GET /events/{eventId}`
- `GET /events/{eventId}/sessions`
- `GET /sessions/{sessionId}`
- `GET /sessions/{sessionId}/drivers`
- `GET /sessions/{sessionId}/teams`
- `GET /sessions/{sessionId}/datasets/{dataset}`
- `GET /sessions/{sessionId}/standings/race`
- `GET /sessions/{sessionId}/broadcast`

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
