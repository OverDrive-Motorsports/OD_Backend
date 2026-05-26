# Gateway Routes

This file lists all routes exposed by the gateway and the internal service they call.

## Auth Behavior

Bearer auth is optional on every route:

- no `Authorization` header: request allowed
- `Authorization` header provided: must be `Bearer <GATEWAY_AUTH_TOKEN>`

## Health Routes

| Gateway Route | Method | Auth | Target | Behavior |
| --- | --- | --- | --- | --- |
| `/health` | `GET` | Optional | Gateway | Returns gateway liveness |
| `/health/auth` | `GET` | Optional | Auth service | Probes auth `/health` |
| `/health/user-data` | `GET` | Optional | User-data service | Probes user-data `/health` |
| `/health/championship` | `GET` | Optional | Championship service | Probes championship `/health` |
| `/health/race-data` | `GET` | Optional | Race-data service | Probes race-data `/health` |
| `/health/ingestion` | `GET` | Optional | Ingestion service | Probes ingestion `/health` |

## Proxied Routes

| Gateway Prefix | Service env target | Path rewrite |
| --- | --- | --- |
| `/auth` | `AUTH_SERVICE_URLS` | prefix stripped |
| `/presets` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| `/providers` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| `/users` | `USER_DATA_SERVICE_URLS` | prefix stripped |
| `/championships` | `CHAMPIONSHIP_SERVICE_URLS` | prefix stripped |
| `/race-data` | `RACE_DATA_SERVICE_URLS` | prefix stripped |
| `/races` | `RACE_DATA_SERVICE_URLS` | prefix stripped |
| `/ingestion` | `INGESTION_SERVICE_URLS` | prefix stripped |

## Forwarded Headers

Gateway forwards and enriches headers:

- `X-Forwarded-Host`
- `X-Forwarded-Proto`
- `X-Forwarded-For`

## Notes

- Gateway target URLs are read from `gateway/.env` (or process env vars), not from hardcoded defaults.
- Multiple URLs in one `*_SERVICE_URLS` variable are load-balanced with round-robin.
- Rate limiting is currently in-memory; counters reset when gateway restarts.
