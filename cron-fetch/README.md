# CronFetch

Tracks companies' career pages, filters their job listings by role/location,
and sends a Telegram message when a genuinely new matching job appears. See
[docs/functional-requirements.md](docs/functional-requirements.md) for how
the service works, [docs/api-reference.md](docs/api-reference.md) for exact
request/response examples (e.g. for building a frontend), and
[docs/non-functional-requirements.md](docs/non-functional-requirements.md)
for an overview of what it does and its current limitations.

## Prerequisites

- Go 1.26+
- Docker Desktop (with Docker Compose)

## Installation

1. Clone the repo and move into it.
2. Create a `.env` file in the project root with:
   ```
   DATABASE_URL=postgres://cronfetch:cronfetch@localhost:5433/cronfetch?sslmode=disable
   JWT_SECRET=some-long-random-string
   TELEGRAM_BOT_TOKEN=your-bot-token
   TELEGRAM_CHAT_ID=your-chat-id
   FETCH_INTERVAL_HOURS=1
   ```
   `DATABASE_URL` and `JWT_SECRET` are required; the app refuses to start
   without them. `TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID` are optional — if
   either is missing, the background job-sync loop is disabled (logged as a
   warning) and everything else still works. `FETCH_INTERVAL_HOURS` defaults
   to `1` if omitted; decimals are allowed (e.g. `0.5` for every 30 minutes).
3. Install Go dependencies:
   ```
   go mod download
   ```

## Database (Docker)

Start Postgres:
```
docker compose up -d
```

Check it's running:
```
docker compose ps
```

Stop it (keeps your data, since it's stored in a Docker volume):
```
docker compose down
```

Connection details (matches `docker-compose.yml`):
- Host: `localhost`
- Port: `5433`
- User: `cronfetch`
- Password: `cronfetch`
- Database: `cronfetch`

Open a psql shell inside the container:
```
docker exec -it cron-fetch-postgres-1 psql -U cronfetch -d cronfetch
```

## Migrations

Migration files live in `migrations/`, applied in order. Run each one
against the running container:
```
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch < migrations/001_create_users_table.sql
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch < migrations/002_add_name_to_users.sql
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch < migrations/003_create_companies_table.sql
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch < migrations/004_add_filters_to_companies.sql
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch < migrations/005_add_platform_to_companies.sql
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch < migrations/006_create_jobs_table.sql
```

When adding a schema change, create a new numbered file rather than editing
an existing one, then apply it the same way. To run migrations against a
different database (e.g. a hosted one like Neon), swap the container's
`psql` for a throwaway one pointed at that database's connection string:
```
docker run --rm -i postgres:16-alpine psql "<connection-string>" < migrations/001_create_users_table.sql
```

Verify a table's structure:
```
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch -c '\d users'
```

## Running the Go project

Make sure Postgres is up and migrated (see above), then:
```
go run ./cmd/api
```

The API listens on `:8080` (or `$PORT` if set — see **Deployment** below).
Check it's alive:
```
curl http://localhost:8080/health
```

If Telegram credentials are set, the background job-sync loop starts
immediately and then runs on `FETCH_INTERVAL_HOURS`. The first sync for any
newly-added company only records a baseline — it won't notify about jobs
that already existed before you started tracking it, only ones that appear
afterward.

## Tests

```
go test ./...
```

Some tests (marked "Live" in their names, e.g. in `internal/fetcher`) call
real career sites and, for Telegram, can send a real message if
`TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID` are set in the environment running
the tests. They need internet access and aren't meant for routine CI.

## Running with Docker

The `Dockerfile` builds a small production image (multi-stage: compiles with
the exact Go version `go.mod` pins, then ships just the binary).

Build it:
```
docker build -t cronfetch:local .
```

Run it, pointed at your local Postgres (note the container needs to reach
Postgres by its container name on the compose network, not `localhost`):
```
docker run --rm -p 8080:8080 \
  --network cron-fetch_default \
  -e DATABASE_URL="postgres://cronfetch:cronfetch@cron-fetch-postgres-1:5432/cronfetch?sslmode=disable" \
  -e JWT_SECRET="some-long-random-string" \
  -e TELEGRAM_BOT_TOKEN="your-bot-token" \
  -e TELEGRAM_CHAT_ID="your-chat-id" \
  cronfetch:local
```

## Deployment

Currently deployed as: **Render** (the app, via the `Dockerfile`) + **Neon**
(hosted Postgres).

- **Neon:** create a project, copy its connection string, and run the
  migrations above against it (using the `docker run ... psql "<connection-string>"`
  form, since there's no local container for a hosted database).
- **Render:** create a Web Service from this repo.
  - **Root Directory:** `cron-fetch` (the Go project lives in this subfolder
    of the git repo, not at its root).
  - **Runtime/Language:** **Docker** — this makes Render build and run the
    `Dockerfile` directly, so the Go version matches `go.mod` exactly
    instead of whatever Render's native Go builder happens to support.
  - **Health Check Path:** `/health`
  - **Environment variables:** set `DATABASE_URL` (the Neon string),
    `JWT_SECRET` (a real random value — as one line, with nothing else in
    the field), `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, and optionally
    `FETCH_INTERVAL_HOURS`. Render supplies `PORT` itself; the app already
    reads it.
- **Keep-alive:** Render's free tier sleeps the app after 15 minutes idle,
  which would also pause the background job-sync loop.
  `.github/workflows/keep-alive.yml` pings `/health` every 10 minutes to
  prevent that — set a repository variable named `APP_URL` (Settings →
  Secrets and variables → Actions → Variables) to your deployed URL (e.g.
  `https://crontfetch.onrender.com`) for it to have something to ping.
