# CronFetch

## Prerequisites

- Go 1.26+
- Docker Desktop (with Docker Compose)

## Installation

1. Clone the repo and move into it.
2. Create a `.env` file in the project root with:
   ```
   TELEGRAM_BOT_TOKEN=your-bot-token
   TELEGRAM_CHAT_ID=your-chat-id
   ```
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

Migration files live in `migrations/`, applied in order. Run each one against the running container:
```
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch < migrations/001_create_users_table.sql
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch < migrations/002_add_name_to_users.sql
```

When adding a schema change, create a new numbered file rather than editing an existing one, then apply it the same way.

Verify a table's structure:
```
docker exec -i cron-fetch-postgres-1 psql -U cronfetch -d cronfetch -c '\d users'
```

## Running the Go project

Make sure Postgres is up (see above), then:
```
go run ./cmd/api
```

The API listens on `:8080`. Check it's alive:
```
curl http://localhost:8080/health
```
