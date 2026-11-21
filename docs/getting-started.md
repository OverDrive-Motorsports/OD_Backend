# OverDrive Backend - Getting Started

This is a from-scratch walkthrough for someone who has never touched this repo: how
to start the backend, load a real F1 race into it, and read that data back out.

It assumes you have `git`, `docker`, `docker compose` and `go` (1.27+: the Go workspace and every
module declare `go 1.27`) installed, and that you've cloned the repo and are running commands from its root.

## 1. What you're starting

The backend is a set of small Go services plus a gateway that sits in front of them:

| Service | Port | What it does |
| --- | ---: | --- |
| `gateway` | `3000` | Single public entrypoint. Everything a client (mobile/AR app, or you) talks to goes through here. |
| `auth-service` | `3001` | Registration, login, refresh, and session records. |
| `user-data-service` | `3002` | AR/VR layout presets of each user (`/presets`, user JWT required). |
| `championship-service` | `3003` | Championship metadata: championships, events, sessions, drivers, teams, standings. |
| `race-data-service` | `3004` | Live race data and telemetry: laps, positions, stints, pit stops, weather, radio, race control, speed/engine/location samples. |
| `ingestion-service` | `3005` | Pulls raw data from the OpenF1 public API and pushes it into the two services above. |
| `postgres` | `5432` | One database per service (`overdrive_championship`, `overdrive_race_data`, ...). |

## 2. Start the stack

First, create the root `.env` from the template:

```bash
cp .env.template .env
```

Every secret ships empty: fill `POSTGRES_PASSWORD`, `GATEWAY_AUTH_TOKEN`, `AUTH_JWT_SECRET` and
`INTERNAL_INGESTION_TOKEN`, generating each value with:

```bash
openssl rand -hex 32
```

There is deliberately no default: the services refuse to start when `AUTH_JWT_SECRET` or
`INTERNAL_INGESTION_TOKEN` is missing or shorter than 32 bytes, and a **new** Postgres volume
refuses an empty password. The failing container exits and `docker compose up` stops with
`dependency failed to start` until the `.env` is complete.

**Upgrading an existing checkout.** Postgres only reads `POSTGRES_PASSWORD` when it creates its
volume: an existing `overdrive-postgres-data` volume keeps the password it was created with.
After pulling this change:
1. Add `INTERNAL_INGESTION_TOKEN` to your `.env`, and regenerate `AUTH_JWT_SECRET` if it is
   shorter than 32 bytes or still `change-me-in-production`.
2. To change the database password, either keep your data and update it in place before
   restarting (replace `postgres` with your `POSTGRES_USER` if you changed it):
   ```bash
   docker compose up -d postgres
   docker compose exec postgres psql -U postgres -c "ALTER USER postgres PASSWORD '<new value from .env>'"
   docker compose up -d --build
   ```
   or start from an empty database (all data lost, re-ingest afterwards):
   `docker compose down -v && docker compose up -d --build`.

Only the gateway (`3000`) is reachable from the network; the service ports (`3001`-`3005`) and
Postgres (`5432`) only answer on `localhost`.

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

`http://localhost:3000/health` is the gateway's own liveness check - it doesn't
require authentication (data routes do, see below).

## 3. Get your tokens

Data routes (`/v1/championship`, `/v1/race-data`, `/v1/presets`) need a **user access
token**. Create an account and keep its token (the register and login routes are
public; the token lasts 15 minutes, log in again to get a new one):

```bash
curl -s -X POST http://localhost:3000/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"dev@example.com","password":"dev-password","username":"Dev"}' > /dev/null
TOKEN=$(curl -s -X POST http://localhost:3000/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"dev@example.com","password":"dev-password"}' | jq -r .token)
```

Ingestion (`/v1/ingestion`) is reserved to administrators. From a script, use the
`GATEWAY_AUTH_TOKEN` value of the root `.env` (never put it in a client app):

```bash
GATEWAY_TOKEN=$(grep '^GATEWAY_AUTH_TOKEN=' .env | cut -d= -f2-)
```

An account can also be promoted to `admin` (see `docs/security.md`); its next login then
returns a token accepted on `/v1/ingestion`.

## 4. Load a real race into the database

The backend doesn't come with any data - you ingest a race from the
[OpenF1](https://openf1.org) public API by calling `ingestion-service` (through the
gateway, under the `/v1/ingestion` prefix). You need an OpenF1 `meeting_key` and
`session_key` for the race you want (the OpenF1 API docs/website let you look these
up; the example below is the 2025 Chinese Grand Prix race).

**Step 1 - catalog, standings, and most race data in one call.** If you omit the
`resources` field entirely, the service ingests everything in the correct
dependency order automatically:

```bash
curl -X POST http://localhost:3000/v1/ingestion/providers/openf1/ingestions \
  -H "Authorization: Bearer $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
        "meeting_key": 1255,
        "session_key": 9998,
        "resources": ["meetings","sessions","drivers","championship_drivers","championship_teams","session_result","starting_grid","laps","position","intervals","stints","pit","weather","team_radio","overtakes","race_control"],
        "dispatch": true
      }'
```

`car_data` and `location` are deliberately left out here - OpenF1 rejects requests
for those two resources across an entire session for every driver at once
("you're likely asking for too much data"). They must be requested one driver at a
time.

**Step 2 - find out which drivers are in the session** (now that `drivers` has been
ingested):

```bash
SESSION_ID="openf1:session:9998"
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/championship/sessions/$SESSION_ID/drivers"
```

Session and driver IDs are **not** the raw OpenF1 numbers - the backend builds its
own stable IDs, always in the form `openf1:session:<session_key>` and
`openf1:f1:driver:<driver_number>`. Read them from an actual response rather than
guessing.

**Step 3 - ingest `car_data`/`location` one driver at a time:**

```bash
for driver in $(curl -s -H "Authorization: Bearer $TOKEN" \
    "http://localhost:3000/v1/championship/sessions/$SESSION_ID/drivers" \
    | grep -o '"driverNumber":[0-9]*' | grep -o '[0-9]*'); do
  echo "ingesting driver $driver..."
  curl -X POST http://localhost:3000/v1/ingestion/providers/openf1/ingestions \
    -H "Authorization: Bearer $GATEWAY_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"meeting_key\":1255,\"session_key\":9998,\"driver_number\":$driver,\"resources\":[\"car_data\",\"location\"],\"dispatch\":true}"
done
```

This loop takes a while - one HTTP round trip to OpenF1 per driver, per resource.
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
next one - that's the intended usage pattern, not a bug if it "hangs" for a while:

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3000/v1/race-data/sessions/$SESSION_ID/race/control"
```

All responses are JSON, camelCase fields, and lists come back as plain arrays
(`[...]`, never wrapped in `{"count": ..., "data": [...]}`).

## 6. Browse the database directly (optional)

Each service has its own Postgres database. To browse them visually with Prisma
Studio (needs Node.js: the script runs `npx prisma@<version>` with the same Prisma version
as the schema sync jobs):

```bash
tools/prisma.sh championship studio --port 5555
tools/prisma.sh race-data studio --port 5556
```

Then open `http://localhost:5555` and `http://localhost:5556` in a browser.

## 7. Stop the stack

```bash
docker compose down
```

Data persists in a Docker volume (`overdrive-postgres-data`) between restarts -
`docker compose down -v` would also wipe it, `docker compose down` alone won't.

## Troubleshooting

- **A request to `/v1/...` returns `401`**: you forgot the `Authorization: Bearer $TOKEN`
  header, or the token is wrong. `TOKEN_EXPIRED` means the 15 minutes are over: log in
  again. The gateway token is refused on data routes (`INVALID_TOKEN`).
- **`/v1/ingestion` returns `403 FORBIDDEN`**: you sent a user token; use
  `$GATEWAY_TOKEN` or an `admin` account's token.
- **`docker compose up` stops with `dependency failed to start`**: a required secret is
  missing or too short (`docker compose logs <service>` names it), or the Postgres password
  in `.env` no longer matches the existing volume - see "Upgrading an existing checkout".
- **A curl call to `car_data`/`location` fails with `422` / "too much data"**: you
  didn't scope it to one `driver_number` - see step 4.
- **You changed some Go code and it's not reflected**: services run via `go run`
  inside their containers, which is not hot-reloading. Restart the affected
  container: `docker compose restart <service-name>`.
- **A service seems stuck on startup**: check `docker compose logs -f <service>` -
  healthchecks have a 60s grace period before Compose gives up on them.
