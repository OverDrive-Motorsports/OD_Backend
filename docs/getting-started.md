# OverDrive Backend — Getting Started

This is a from-scratch walkthrough for someone who has never touched this repo: how
to start the backend, load a real F1 race into it, and read that data back out.

It assumes you have `git`, `docker`, `docker compose`, `go` (1.25+ — the Go workspace
declares `go 1.25.0`), and `node`/`npm` installed, and that you've cloned the repo and are running commands from its root.

## 1. What you're starting

The backend is a set of small Go services plus a gateway that sits in front of them:

| Service | Port | What it does |
| --- | ---: | --- |
| `gateway` | `3000` | Single public entrypoint. Everything a client (mobile/AR app, or you) talks to goes through here. |
| `auth-service` | `3001` | Registration, login, refresh, and session records. |
| `user-data-service` | `3002` | User presets/providers/widgets. Still scaffolding — health check only for now. |
| `championship-service` | `3003` | Championship metadata: championships, events, sessions, drivers, teams, standings. |
| `race-data-service` | `3004` | Live race data and telemetry: laps, positions, stints, pit stops, weather, radio, race control, speed/engine/location samples. |
| `ingestion-service` | `3005` | Pulls raw data from the OpenF1 public API and pushes it into the two services above. |
| `postgres` | `5432` | One database per service (`overdrive_championship`, `overdrive_race_data`, ...). |

## 2. Start the stack

First, create the root `.env` from the template:

```bash
cp .env.template .env
```

`AUTH_JWT_SECRET` must be set in there — `auth-service` refuses to start without it
(there is deliberately no insecure default), so leaving it empty makes that container
crash-loop.

Then, from the repo root:

```bash
docker compose up -d
```

This starts Postgres, runs the four Prisma schema sync jobs, then starts all five
services and `gateway` in the right order (each waits for its dependencies to be
healthy).

Check everything is up:

```bash
docker compose ps
curl -fsS http://localhost:3000/health
```

`http://localhost:3000/health` is the gateway's own liveness check — it doesn't
require authentication (every other route does, see below).

## 3. Get the gateway's auth token

Every route under `/v1/...` requires a `Authorization: Bearer <token>` header. In
this dev setup, the token is a fixed value read from the root `.env`
(`GATEWAY_AUTH_TOKEN`, `admin` in `.env.template`):

```bash
TOKEN=admin
```

If you ever change that value in `.env`, update this everywhere you see `$TOKEN`
below and restart the gateway.

## 4. Load a real race into the database

The backend doesn't come with any data — you ingest a race from the
[OpenF1](https://openf1.org) public API by calling `ingestion-service` (through the
gateway, under the `/v1/ingestion` prefix). You need an OpenF1 `meeting_key` and
`session_key` for the race you want (the OpenF1 API docs/website let you look these
up; the example below is the 2025 Chinese Grand Prix race).

**Step 1 — catalog, standings, and most race data in one call.** If you omit the
`resources` field entirely, the service ingests everything in the correct
dependency order automatically:

```bash
curl -X POST http://localhost:3000/v1/ingestion/providers/openf1/ingestions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
        "meeting_key": 1255,
        "session_key": 9998,
        "resources": ["meetings","sessions","drivers","championship_drivers","championship_teams","session_result","starting_grid","laps","position","intervals","stints","pit","weather","team_radio","overtakes","race_control"],
        "dispatch": true
      }'
```

`car_data` and `location` are deliberately left out here — OpenF1 rejects requests
for those two resources across an entire session for every driver at once
("you're likely asking for too much data"). They must be requested one driver at a
time.

**Step 2 — find out which drivers are in the session** (now that `drivers` has been
ingested):

```bash
SESSION_ID="openf1:session:9998"
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/championship/sessions/$SESSION_ID/drivers"
```

Session and driver IDs are **not** the raw OpenF1 numbers — the backend builds its
own stable IDs, always in the form `openf1:session:<session_key>` and
`openf1:f1:driver:<driver_number>`. Read them from an actual response rather than
guessing.

**Step 3 — ingest `car_data`/`location` one driver at a time:**

```bash
for driver in $(curl -s -H "Authorization: Bearer $TOKEN" \
    "http://localhost:3000/v1/championship/sessions/$SESSION_ID/drivers" \
    | grep -o '"driverNumber":[0-9]*' | grep -o '[0-9]*'); do
  echo "ingesting driver $driver..."
  curl -X POST http://localhost:3000/v1/ingestion/providers/openf1/ingestions \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"meeting_key\":1255,\"session_key\":9998,\"driver_number\":$driver,\"resources\":[\"car_data\",\"location\"],\"dispatch\":true}"
done
```

This loop takes a while — one HTTP round trip to OpenF1 per driver, per resource.
That's expected; it's bounded by the external API, not your machine.

## 5. Read the race back out

Everything below goes through the gateway with the same `Authorization` header.
Replace `$SESSION_ID` / driver number `63` with whatever you actually ingested.

**Championship metadata:**

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:3000/v1/championship/championships

curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/championship/sessions/$SESSION_ID/standings"

curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/championship/drivers/63/profile?championshipCode=f1"
```

**Live race data:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/race-data/sessions/$SESSION_ID/race/laps?driverNumber=63"

curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/race-data/sessions/$SESSION_ID/race/stints"

curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/race-data/sessions/$SESSION_ID/race/weather"
```

**Telemetry:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/race-data/sessions/$SESSION_ID/drivers/63/telemetry/speed"
```

**Race control, as a long-poll:** this call blocks (up to 30 seconds) until a new
race control event arrives, then returns and closes. Call it again to wait for the
next one — that's the intended usage pattern, not a bug if it "hangs" for a while:

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/race-data/sessions/$SESSION_ID/race/control"
```

All responses are JSON, camelCase fields, and lists come back as plain arrays
(`[...]`, never wrapped in `{"count": ..., "data": [...]}`).

## 6. Browse the database directly (optional)

Each service has its own Postgres database. To browse them visually with Prisma
Studio:

```bash
npm install   # once, from repo root

CHAMPIONSHIP_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/overdrive_championship?schema=public' \
npx prisma studio --schema services/championship-service/resources/schema.prisma --port 5555

RACE_DATA_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/overdrive_race_data?schema=public' \
npx prisma studio --schema services/race-data-service/resources/schema.prisma --port 5556
```

Then open `http://localhost:5555` and `http://localhost:5556` in a browser.

## 7. Stop the stack

```bash
docker compose down
```

Data persists in a Docker volume (`overdrive-postgres-data`) between restarts —
`docker compose down -v` would also wipe it, `docker compose down` alone won't.

## Troubleshooting

- **A request to `/v1/...` returns `401`**: you forgot the `Authorization: Bearer
  admin` header, or the token is wrong.
- **A curl call to `car_data`/`location` fails with `422` / "too much data"**: you
  didn't scope it to one `driver_number` — see step 4.
- **You changed some Go code and it's not reflected**: services run via `go run`
  inside their containers, which is not hot-reloading. Restart the affected
  container: `docker compose restart <service-name>`.
- **A service seems stuck on startup**: check `docker compose logs -f <service>` —
  healthchecks have a 60s grace period before Compose gives up on them.
