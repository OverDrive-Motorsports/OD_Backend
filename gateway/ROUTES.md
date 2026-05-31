# Gateway Routes

This file lists routes exposed by the gateway and their upstream service mapping.

## Auth Behavior

Bearer auth is optional on every route:

- no `Authorization` header: request allowed
- `Authorization` header provided: must be `Bearer <GATEWAY_AUTH_TOKEN>`

If an invalid Authorization header is provided, gateway returns:

```json
{
  "error": "unauthorized"
}
```

## Request Headers

Client headers accepted by gateway:

- `Authorization` (optional): `Bearer <GATEWAY_AUTH_TOKEN>`
- `Content-Type`: forwarded as received
- `Accept`: forwarded as received

Forwarded/proxy headers added by gateway:

- `X-Forwarded-Host`
- `X-Forwarded-Proto`
- `X-Forwarded-For`

## Health Routes

| Gateway Route | Method | Auth | Target | Behavior |
| --- | --- | --- | --- | --- |
| `/health` | `GET` | Optional | Gateway | Returns gateway liveness |
| `/health/auth` | `GET` | Optional | Auth service | Probes auth `/health` |
| `/health/user-data` | `GET` | Optional | User-data service | Probes user-data `/health` |
| `/health/championship` | `GET` | Optional | Championship service | Probes championship `/health` |
| `/health/race-data` | `GET` | Optional | Race-data service | Probes race-data `/health` |
| `/health/ingestion` | `GET` | Optional | Ingestion service | Probes ingestion `/health` |

## Proxied Prefixes By Domain

| Domain | Gateway Prefix | Service env target | Path rewrite |
| --- | --- | --- | --- |
| Auth | `/auth` | `AUTH_SERVICE_URLS` | prefix stripped |
| User data | `/presets` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| User data | `/providers` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| User data | `/users` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| Championship | `/championships` | `CHAMPIONSHIP_SERVICE_URLS` | prefix stripped |
| Championship v1 facade | `/v1/championship` | `CHAMPIONSHIP_SERVICE_URLS` | prefix stripped |
| Race data | `/race-data` | `RACE_DATA_SERVICE_URLS` | prefix stripped |
| Race data | `/races` | `RACE_DATA_SERVICE_URLS` | prefix stripped |
| Ingestion (internal) | `/ingestion` | `INGESTION_SERVICE_URLS` | prefix stripped |

## Championship V1 Public Endpoints

All routes below are exposed by gateway under `/v1/championship/*` and proxied to `championship-service`.

### Catalog

| Gateway | Upstream |
| --- | --- |
| `GET /v1/championship/championships` | `GET /championships` |
| `GET /v1/championship/championships/{code}/events` | `GET /championships/{code}/events` |
| `GET /v1/championship/events/{eventId}` | `GET /events/{eventId}` |
| `GET /v1/championship/events/{eventId}/sessions` | `GET /events/{eventId}/sessions` |

### Session

| Gateway | Upstream |
| --- | --- |
| `GET /v1/championship/sessions/{sessionId}` | `GET /sessions/{sessionId}` |
| `GET /v1/championship/sessions/{sessionId}/drivers` | `GET /sessions/{sessionId}/drivers` |
| `GET /v1/championship/sessions/{sessionId}/teams` | `GET /sessions/{sessionId}/teams` |
| `GET /v1/championship/sessions/{sessionId}/broadcast` | `GET /sessions/{sessionId}/broadcast` |
| `GET /v1/championship/sessions/{sessionId}/standings/race` | `GET /sessions/{sessionId}/standings/race` |

### Datasets

| Gateway | Upstream |
| --- | --- |
| `GET /v1/championship/sessions/{sessionId}/datasets/{dataset}` | `GET /sessions/{sessionId}/datasets/{dataset}` |

Gateway validates `{dataset}` against the following list before proxying:

- `session_result`
- `starting_grid`
- `championship_drivers`
- `championship_teams`

If `{dataset}` is not in this list, gateway returns:

- status `400`
- body `{ "error": "invalid parameter" }`

## Error Handling Notes

- Upstream service responses are passed through as-is (including status code and body).
- Gateway-generated errors include:
  - `401` invalid provided auth header
  - `429` rate limit exceeded
  - `400` invalid championship dataset parameter on v1 dataset route
  - `502` upstream unavailable/proxy failure

## Notes

- Gateway target URLs are read from `gateway/.env` (or process env vars), not from hardcoded defaults.
- Multiple URLs in one `*_SERVICE_URLS` variable are load-balanced with round-robin.
- Rate limiting is currently in-memory; counters reset when gateway restarts.
