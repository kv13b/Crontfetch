# Functional Requirements — CronFetch

This document lists every route, the services behind them, and what each is
responsible for. For how to run the project and try these yourself, see the
main [README.md](../README.md).

## Auth

All tokens are JWTs signed with `JWT_SECRET`, valid for 24 hours.

### `POST /signup`
Creates a user account and returns a token.

- **Body:** `{name, email, password}` — password must be at least 8 characters.
- **Response (201):** `{token, user: {id, name, email, created_at, updated_at}}`
- **Errors:** `400` missing/invalid fields, `409` email already registered.

### `POST /login`
Verifies credentials and returns a token.

- **Body:** `{email, password}`
- **Response (200):** `{token, user}`
- **Errors:** `400` missing fields, `401` invalid email or password (the same
  message for both cases, so a caller can't tell which one is wrong).

### `GET /me` (requires `Authorization: Bearer <token>`)
Returns the authenticated caller's own profile, looked up fresh from the
database by the user ID embedded in the token.

- **Response (200):** the `user` object.
- **Errors:** `401` missing/invalid/expired token.

## Companies

A company is a career page a user tracks, together with the filters that
decide which of its jobs are relevant. All company routes require
`Authorization: Bearer <token>` and are scoped to the caller — one user can
never see, edit, or fetch jobs for another user's company.

### `POST /companies`
Adds a career page to track.

- **Body:**
  ```json
  {
    "name": "Cloudflare",
    "career_url": "https://www.cloudflare.com/careers/jobs/",
    "platform": "greenhouse",
    "board": "cloudflare",
    "min_experience_years": 2,
    "max_experience_years": 6,
    "roles": ["software engineer", "developer"],
    "locations": ["Bangalore, India", "Remote"]
  }
  ```
  Only `name` and `career_url` are required. `platform`/`board` are optional —
  see **Platform detection** below. Every filter field is optional; an absent
  or empty one means "no constraint" on that dimension.
- **Response (201):** the saved company, including the detected/validated
  `platform` and `board`.
- **Errors:** `400` invalid input (see validation rules below), `409` this
  user already tracks this `career_url`.

### `GET /companies`
Lists the caller's tracked companies, most recently created first.

### `PATCH /companies/{id}`
Partially updates a company — only the fields present in the body change.

- **Body:** any subset of the `POST /companies` fields.
- Sending `career_url` without also sending `platform`/`board` re-runs
  platform detection against the new URL.
- Sending `null` for `min_experience_years`/`max_experience_years` clears
  that limit; omitting the field leaves it unchanged.
- `roles`/`locations` are replaced wholesale, not merged — send `[]` to
  clear one.
- Unknown JSON fields are rejected (`400`), so a typo doesn't silently no-op.
- **Errors:** `400` empty body / no fields / same validation as create,
  `404` no such company (or it belongs to someone else), `409` the new
  `career_url` collides with another company this user already tracks.

### `GET /companies/{id}/jobs`
Fetches the company's current job listings live and returns only the ones
matching its saved `roles`/`locations` filters.

- **Response (200):**
  ```json
  {
    "company": "Cloudflare",
    "total_scanned": 395,
    "total_matched": 38,
    "warnings": ["experience filters are saved but not applied yet: ..."],
    "jobs": [{"id": "...", "title": "...", "location": "...", "url": "..."}]
  }
  ```
  `warnings` appears when experience filters are set, since they are stored
  but not yet used to filter (see [non-functional-requirements.md](non-functional-requirements.md)).
- **Errors:** `404` no such company, `422` the career site's platform
  couldn't be read (unrecognised, or a recognised-but-unsupported vendor
  such as Taleo), `422` a saved board name doesn't exist on that platform.

### Validation rules (create and update)
- `name`, `career_url` required (create only — update requires at least one field).
- `career_url` must start with `http://` or `https://`.
- `platform`, if set, must be one of the supported platforms (below).
- `board` requires `platform` to be set alongside it.
- Platforms that need a board name (all except TalentBrew and Workday)
  require one, unless `career_url` itself reveals it (e.g. a
  `boards.greenhouse.io/<board>` link).
- `beesite`'s `board` must be an `https://` URL (the platform's own job API address).
- `workday`'s `career_url` must be a `*.myworkdayjobs.com` link.
- `min_experience_years`/`max_experience_years`, if set, must be non-negative,
  and min can't exceed max.

## Job fetching and platform detection

`GET /companies/{id}/jobs` and the background sync (below) both go through
the same fetch pipeline:

1. **Detect the platform**, if not already saved — from the `career_url`
   directly (e.g. `boards.greenhouse.io`, `*.myworkdayjobs.com`), or by
   fetching the page and looking for markers in its HTML (embed scripts,
   job links, a platform's tracking parameters), or — for Greenhouse only —
   by guessing a board name from the domain and confirming it against the
   board's own registered company name.
2. **Fetch jobs** from that platform using its native API where one exists,
   or by scraping (TalentBrew).
3. **Filter** the results by the company's saved `roles` (case-insensitive
   substring match against the job title) and `locations` (matched against
   the job's location, with special handling for "Remote").

### Supported platforms
TalentBrew, Greenhouse, Workday, Lever, Ashby, SmartRecruiters, Workable,
Recruitee, BeeSite.

A handful of other platforms (Taleo, iCIMS, SuccessFactors, Oracle
Recruiting, Teamtailor, Personio, Jobvite) are *recognised* by their markers
but not yet fetchable — the API reports which one by name instead of a
generic failure.

## Background job sync and notifications

A background loop (started with the API server) runs on a timer
(`FETCH_INTERVAL_HOURS`, default 1 hour) and, for every tracked company
across all users:

1. Fetches and filters its jobs, same as `GET /companies/{id}/jobs`.
2. Compares them against a `jobs` table recording every job ever seen for
   that company.
3. **First sync for a company:** records its current matches as a baseline,
   without notifying — otherwise tracking a company with an existing
   backlog of matches would immediately flood Telegram.
4. **Every sync after that:** sends a Telegram message for each job that
   wasn't recorded before, then records it.

Notifications go to a single fixed Telegram chat (`TELEGRAM_CHAT_ID`), not
per-user — see [non-functional-requirements.md](non-functional-requirements.md).

## `GET /health`
Liveness/readiness check. Returns `{"status": "online", "db": "online"|"unreachable"}`,
pinging the database on every call.
