# OpenF1 live data: access terms and tuning

Summary of the terms published by OpenF1 (`https://openf1.org`, `https://openf1.org/auth.html`,
`https://openf1.org/docs`). They may change: check them before relying on live data.

| Topic | Terms |
| --- | --- |
| Historical data | free, no authentication, seasons from 2023 |
| Live data | paid subscription and authentication; data is live from 30 minutes before a session starts to 30 minutes after it ends |
| Authentication | OAuth2: `POST https://api.openf1.org/token` with the account `username` and `password` (form), answers an `access_token` valid one hour, sent as `Authorization: Bearer`. There is no static API key |
| Rate limits | free: 3 requests per second and 30 per minute; subscription: 6 per second and 60 per minute |
| Delay | about 3 seconds behind the event |
| Licence | CC BY-NC-SA 4.0, non-commercial analysis and fan engagement; other uses need an agreement with OpenF1. Not affiliated with Formula 1 |

OpenF1 recommends MQTT or WebSocket for live data. This service polls the REST API: no new
dependency, and the same code path as a manual ingestion.

## Account

`OPENF1_USERNAME` and `OPENF1_PASSWORD` (both or neither, no default) are exchanged for an access
token, cached until shortly before it expires, renewed once when OpenF1 refuses it, and sent on
every OpenF1 call, manual ingestions included. Without an account the calls are anonymous: past
sessions work, live ones end in `OpenF1 refused the credentials` or `OpenF1 request failed`.
`OPENF1_TOKEN_URL` (default `https://api.openf1.org/token`) must be an https URL, and so must
`OPENF1_BASE_URL` when an account is set.

## Polling

Each resource is polled every few poll intervals, with the time filter of OpenF1
(`date>=<latest known row minus an overlap>`):

| Resource | Every | With the default `3s` |
| --- | --- | --- |
| `race_control`, `position`, `location`, `car_data` | 2 intervals | 6 s |
| `intervals` | 4 | 12 s |
| `laps` (`date_start`) | 5 | 15 s |
| `pit` | 10 | 30 s |
| `team_radio`, `weather`, `stints` (`stints` has no time field and is fetched whole) | 20 | 60 s |

That is 54 calls a minute, under the 60 of a subscription. Polling everything every 3 seconds would
be 200. Without an account, use `OPENF1_LIVE_POLL_INTERVAL=6s` (27 calls a minute). A `429` waits
for its `Retry-After`.

| Variable | Default | Meaning |
| --- | --- | --- |
| `OPENF1_LIVE_POLL_INTERVAL` | `3s` | base poll interval |
| `OPENF1_LIVE_GRACE` | `30m` | how long after the scheduled session end a job goes on |
| `OPENF1_LIVE_MAX_FAILURES` | `20` | rounds in a row where every poll failed before the job gives up: after that many rounds when OpenF1 refuses the credentials, otherwise only once the session end and grace period have passed (an outage during the session is waited out); past the end, also the failed polls in a row of one resource before the job stops waiting for it and ends `failed` |
| `OPENF1_LIVE_MAX_JOBS` | `1` | live jobs running at once; they share the rate limit |

A set but invalid value refuses to start the service.
