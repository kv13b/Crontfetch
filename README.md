
# Cronfetch

CronFetch is a full-stack automated career page tracker that scans company boards, filters relevant jobs, and delivers instant alerts.

## Features

Manual & Automated Sync: Trigger immediate job scans on demand via the Sync Jobs Now (POST /run-now) button or let the backend run automated background scraping intervals.

Telegram Notifications: Automatically pushes matching job postings directly to your Telegram chat.

Map-Based Company Discovery (Proposal): Automatically discover and track companies and their career portals based on local geographic regions using OpenStreetMap data when you go online.

Multi-Platform Support: Native scrapers and auto-detectors for popular applicant tracking systems including Greenhouse, Workday, Lever, Ashby, SmartRecruiters, TalentBrew, Workable, Recruitee, and BeeSite.

Granular Filtering: Set custom filters per company for role titles, geographic locations, and minimum/maximum years of experience.

Secure Authentication & Management: Full JWT-based user signup/login flows alongside complete company CRUD (Create, Read, Update, Delete) dashboards.


## Installation

Configure environment variables in a .env file:

```bash
PORT=8080
DATABASE_URL=postgres://user:password@localhost:5432/cronfetch
JWT_SECRET=your_jwt_secret
CORS_ALLOWED_ORIGINS=http://localhost:5173
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id
```
## Frontend (React/Vite):
```bash
npm install
npm run dev
```
## Run with Docker:
```bash
docker-compose up --build
```
## Authors

- [@kv13b](https://www.github.com/octokatherine)

