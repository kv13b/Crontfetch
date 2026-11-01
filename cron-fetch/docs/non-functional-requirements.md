# Non-Functional Requirements — CronFetch

## What this application does

CronFetch is a personal job-alert tool. A user tracks the career pages of
companies they're interested in, along with filters (role keywords,
location, and an experience range). Instead of manually re-checking those
pages, CronFetch periodically fetches each one's current listings, works out
which are newly posted since the last check, and sends a Telegram message
for each match — the goal being to hear about a relevant opening within
about an hour of it going live, without visiting a single career site by
hand.

For the full list of endpoints and exactly what each does, see
[functional-requirements.md](functional-requirements.md).

## Architecture at a glance

- **Language/runtime:** Go, a single binary (`cmd/api`) that serves the HTTP
  API and runs the background sync loop in the same process.
- **Database:** PostgreSQL, accessed via `pgx`. Schema changes are plain
  `.sql` files in `migrations/`, applied by hand in the order they're
  numbered (no migration tool wired in yet).
- **External services:** each supported career platform's own API (or, for
  TalentBrew, HTML scraping), the Telegram Bot API for notifications, and
  free OpenStreetMap data: Nominatim (address → coordinates), a state
  extract imported with `cmd/importplaces` (offices near a point), and the
  public Overpass and Photon servers as fallbacks for areas not imported.
  No paid map or geocoding API is used.
- **Auth:** stateless JWTs (HS256), no server-side session store.

## Performance

- **On-demand job fetches are slow for some platforms.** `GET
  /companies/{id}/jobs` scrapes the career site live on every call. Fast
  JSON-API platforms (Greenhouse, Lever, Ashby, etc.) typically respond in
  under a second; TalentBrew and Workday, which require many sequential
  pages, can take 8–17 seconds per request. This is acceptable for the
  background sync (which isn't time-sensitive), but a poor fit for a
  request a human is waiting on.
- **Large tenants are truncated, not paginated fully.** Fetches cap at 100
  pages per company per request. A company with more listings than that
  (observed with NVIDIA's Workday tenant, over 2,000 jobs) will silently
  miss the excess rather than error.
- **The background sync serializes**, not runs concurrently: one full pass
  over every tracked company must finish before the timer's next tick is
  honored, so a very large number of tracked companies (or several slow
  ones) can make the effective check interval longer than configured.

- **Map lookups are rate-limited by design.** Nominatim allows one request
  per second, so the server queues address lookups and spaces them out.
  `/nearby/places` reads imported offices from the database (indexed,
  instant). Only areas not imported go to the public servers: one lookup
  at a time, cached for 24 hours per ~5 km grid cell, and a server that
  fails is skipped for 10 minutes. Those servers are often overloaded: in
  testing, all four Overpass mirrors timed out for long stretches, and
  Photon stopped answering after a burst of about 60 requests. That's why
  the import comes first. `/nearby` is fast: it only reads the database
  and computes distances in memory.
- **The import is a full re-read of the extract.** Karnataka is about
  130 MB and is read twice (nodes, then building outlines). Re-run it now
  and then to pick up new offices; the extracts are updated daily.

## Reliability

- **One company's failure doesn't stop the sync.** `SyncAll` logs and moves
  on if fetching or notifying for one company errors, so an unsupported or
  temporarily-down career site can't block updates for the rest.
- **No retry/backoff** on a failed fetch or a failed Telegram send — it's
  picked up again next cycle at the earliest, and a job that failed to
  notify (but was already recorded) will never be retried, since the
  "already seen" record was still written.
- **No transactional guarantee across "record" and "notify".** A crash
  between recording a job and sending its Telegram message would leave it
  recorded but un-notified.

## Security

- **Passwords** are hashed with bcrypt; never stored or logged in plaintext.
- **JWTs** expire after 24 hours; no refresh-token flow exists (a user
  simply logs in again).
- **No rate limiting** on any endpoint, including `/signup` and `/login`.
- **No SSRF protection.** The server fetches whatever URL a user supplies
  (`career_url`, and for BeeSite the `board` API address) with no check
  against internal/private network addresses. This is an accepted gap for a
  single-user personal tool, but would need fixing (a dialer guard against
  private IP ranges) before this is exposed to untrusted users.
- **Secrets** (`JWT_SECRET`, `TELEGRAM_BOT_TOKEN`, `DATABASE_URL`) are read
  from environment variables / `.env`, never committed.
- **Location privacy.** The user's own position is only sent as query
  parameters to `/nearby` and `/nearby/places`; it isn't stored. Only
  company office locations are saved.

## Scalability and multi-tenancy

- **Notifications are not per-user.** A single fixed `TELEGRAM_CHAT_ID`
  receives every notification for every user's tracked companies. This
  matches the tool's current scope (one person's own use), but would need a
  `telegram_chat_id` per user (and its own opt-in/verification flow) to
  support multiple people meaningfully.
- **The `jobs` table grows without bound.** Nothing prunes old job records,
  even for postings long since filled or removed. `jobs.last_seen_at` is now
  kept up to date, which is the basis for pruning later.
- **The public-server cache is in memory.** It's lost on restart and not
  shared between instances. Imported offices are in Postgres and shared.
- **No horizontal scaling story.** A single process serves the API and runs
  the sync loop; running more than one instance would sync (and notify)
  duplicately, since nothing coordinates between instances.

## Observability

- **Structured logging** via `log/slog` — every HTTP request logs method,
  path, status, and duration; sync cycles log per-company results
  (baseline vs. synced, matched/new counts) and errors with context.
- **No metrics or alerting** beyond logs — nothing pages if the sync loop
  stalls or Telegram sends start failing.

## Known functional gaps (tracked, not yet built)
- Experience-range filters (`min/max_experience_years`) are stored and
  validated but not applied to job matching — doing so needs fetching each
  shortlisted job's full description and parsing free text for a stated
  range.
- `/nearby` searches only the caller's own tracked companies; there is no
  shared company directory yet.
- `jobs.closed_at` exists but is never set: the sync only sees jobs that
  pass the company's filter, so it can't tell "closed" from "no longer
  matching".
- Several career platforms are recognised but not fetchable (see
  functional-requirements.md).
