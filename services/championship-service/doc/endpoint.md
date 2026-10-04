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
- `GET /sessions/{sessionId}/broadcast` — `{ "sessionId", "feeds": Feed[] }` (see "Video feeds" below)
- `PUT /sessions/{sessionId}/broadcast` — replaces the session feed list (see "Video feeds" below)
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

## Video feeds (`Feed` model)

Video for a session is described by **feed descriptors**, never by playable DRM
URLs and never by a user token. The same `Feed` object is used by
`championship-service` (global session feeds: main broadcast, pit lane, ...)
and `race-data-service` (one onboard camera list per driver).

```json
{ "provider": "f1tv",    "contentId": "1000005432", "channelId": "1017", "label": "Onboard", "startedAtUtc": "2026-07-06T14:02:51Z" }
{ "provider": "youtube", "url": "https://www.youtube.com/watch?v=...", "label": "Highlights" }
{ "provider": "hls",     "url": "https://cdn.example.com/demo/index.m3u8", "label": "Demo" }
```

| Field | Rule |
| --- | --- |
| `provider` | Required. One of `f1tv`, `youtube`, `hls` (exact, lower-case). |
| `contentId`, `channelId` | Required for `f1tv` (non-empty, at most 64 characters each). Forbidden for `youtube` / `hls`. |
| `url` | Required for `youtube` / `hls`, forbidden for `f1tv`. Absolute `https` URL, at most 2048 characters. `youtube`: host must be `youtube.com`, `www.youtube.com`, `m.youtube.com` or `youtu.be`. `hls`: path must end with `.m3u8`. |
| `label` | Optional, at most 80 characters. |
| `startedAtUtc` | Optional. RFC 3339 timestamp of the video's **first frame** (any offset is accepted and stored/returned in UTC, sub-second precision kept). Must fall within the session window widened by 6 h on each side — or within 24 h after the start when the session has no end time yet. See "Synchronising video and race data" below. |

A list holds **at most 20 feeds** and **no duplicate** (same `provider` +
`url`, or same `provider` + `contentId` + `channelId`). Unknown JSON fields are
rejected. A violation is answered with `400`
`{ "error": { "code": "VALIDATION_ERROR", "status": 400, "message": "invalid feed: feeds[<index>].<field> ..." } }`
naming the failing entry and field. In particular a `url` on an `f1tv` feed is
always rejected: the backend never accepts, stores or logs an F1 TV manifest,
licence URL or token.

Feed lists are only written through the `PUT` endpoints below. Ingestion
creates rows with an empty list and **never overwrites** an existing list on
re-ingestion.

### How the frontend resolves each provider (reference)

- `youtube` / `hls`: play the `url` directly (embed / native HLS player).
- `f1tv`: the app logs the user into F1 TV on the device (token kept on the
  device only, never sent to OverDrive), reads `contentId` + `channelId` from
  the backend, then calls the F1 TV play endpoint with its own token to obtain
  the signed DASH manifest and Widevine / FairPlay licence for a DRM-capable
  player. Users without an F1 TV account simply skip `f1tv` feeds. DRM
  playback renders in a protected surface: it can be shown in a native overlay
  but not mapped as a texture on a 3D object.

### Synchronising video and race data

Every race-data sample (`/race/replay`, `/race/*`, `/telemetry/*`) is stamped in
UTC. `startedAtUtc` ties a video to that timeline:

```
dataTimeUtc = startedAtUtc + playbackPosition
```

Example: a feed with `"startedAtUtc": "2026-07-06T14:02:51Z"` played at
`00:12:30` shows the track at `2026-07-06T14:15:21Z`; the client displays the
replay samples whose `timestamp` is the latest one ≤ that instant.

- **Playing**: advance `dataTimeUtc` with the player clock; do not re-query the
  backend per frame — load `/race/replay` once and index it by timestamp.
- **Scrubbing / seeking**: recompute `dataTimeUtc` from the new playback
  position and jump to the matching sample (binary search on the sorted
  timestamps); samples are not interpolated server-side.
- **No `startedAtUtc`**: play the video without overlay.
- **Live streams**: the value is the wall-clock time of the stream's first
  frame; broadcast latency is not compensated server-side — the player should
  expose a user-adjustable offset.

### `GET /sessions/{sessionId}/broadcast`

Returns the session's global feed list. `feeds` is always a JSON array (`[]`
when nothing is registered), never `null`. Unknown session: `404`
`SESSION_NOT_FOUND`.

```json
{ "sessionId": "openf1:session:9998", "feeds": [] }
```

`feeds` is also present on every session summary (`GET /sessions/{sessionId}`,
`GET /events/{eventId}/sessions`).

### `PUT /sessions/{sessionId}/broadcast`

Replaces the **whole** feed list of the session. Body (at most 64 KiB):

```json
{ "feeds": [ { "provider": "f1tv", "contentId": "1000005432", "channelId": "1017", "label": "World feed", "startedAtUtc": "2026-07-06T14:02:51Z" } ] }
```

`{ "feeds": [] }` clears the list. Responses:

- `200` — the updated payload, same shape as the `GET`
- `400` `VALIDATION_ERROR` — malformed body (including a `startedAtUtc` that is
  not RFC 3339), missing `feeds`, unknown field, or a feed violating the rules
  above (message names `feeds[<index>].<field>`, e.g. `feeds[0].startedAtUtc
  must be between ... and ...`)
- `404` `SESSION_NOT_FOUND` — unknown session

```bash
curl -X PUT http://localhost:3003/sessions/<SESSION_ID>/broadcast \
  -H 'Content-Type: application/json' \
  -d '{"feeds":[{"provider":"youtube","url":"https://www.youtube.com/watch?v=abc","label":"Highlights"}]}'
```

### Example

```bash
curl http://localhost:3003/championships
curl http://localhost:3003/championships/f1/events
curl http://localhost:3003/events/<EVENT_ID>/sessions
curl http://localhost:3003/sessions/<SESSION_ID>/datasets/session_result
curl http://localhost:3003/sessions/<SESSION_ID>/datasets/starting_grid
```
