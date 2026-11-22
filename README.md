
# Cronfetch

CronFetch is a full-stack automated career page tracker that scans company boards, filters relevant jobs, and delivers instant alerts.

## Features

**Automated Sync:** The backend scans every tracked company on a schedule (`FETCH_INTERVAL_HOURS`, default 1 hour) and records each job it finds, so only genuinely new postings trigger an alert.

**Telegram Notifications:** Automatically pushes new matching job postings directly to your Telegram chat.

**Jobs Around Me:** A map of company offices near your location, built on free OpenStreetMap data. Track a nearby company in one click: CronFetch looks up its career page for you.

**Profile Preferences:** Save the locations, roles, areas of interest and experience range you're looking for on your account (`GET`/`PUT /me/preferences`).

**Multi-Platform Support:** Native scrapers and auto-detectors for popular applicant tracking systems including Greenhouse, Workday, Lever, Ashby, SmartRecruiters, TalentBrew, Workable, Recruitee, and BeeSite.

**Granular Filtering:** Set custom filters per company for role titles, geographic locations, and minimum/maximum years of experience. The experience filter currently applies to Greenhouse, Lever, Ashby, Workable and Recruitee.

**Secure Authentication & Management:** JWT-based signup/login alongside a dashboard to add, edit and delete tracked companies.

**Sync Now:** A dashboard button (`POST /run-now`) that checks all your companies immediately and sends Telegram alerts for new jobs. Limited to once a minute.

**In progress:** Search jobs page.

## Project structure

| Folder | What it is |
|---|---|
| `cron-fetch/` | Go backend API, background sync and migrations |
| `crontfetch-web/` | React + Vite frontend |
| `docs/` | Execution plan and design notes |

## Installation

### 1. Database

Start Postgres with Docker (exposed on port 5433):

```bash
cd cron-fetch
docker-compose up -d
```

Then apply every file in `cron-fetch/migrations/` in order, as listed in [cron-fetch/README.md](cron-fetch/README.md#migrations).

### 2. Backend (Go)

Create `cron-fetch/.env`:

```bash
PORT=8080
DATABASE_URL=postgres://cronfetch:cronfetch@localhost:5433/cronfetch
JWT_SECRET=your_jwt_secret
CORS_ALLOWED_ORIGINS=http://localhost:5173
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id
FETCH_INTERVAL_HOURS=1
```

Background sync is disabled unless both Telegram variables are set.

```bash
cd cron-fetch
go run ./cmd/api
```

### 3. Frontend (React/Vite)

```bash
cd crontfetch-web
npm install
npm run dev
```

Open http://localhost:5173.

## Docs

- API reference: [cron-fetch/docs/api-reference.md](cron-fetch/docs/api-reference.md)
- Maps and Jobs around me: [docs/maps-and-nearby.md](docs/maps-and-nearby.md)

## Authors

- [@kv13b](https://www.github.com/kv13b)
