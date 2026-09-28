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
  TalentBrew, HTML scraping), plus the Telegram Bot API for notifications.
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

## Scalability and multi-tenancy

- **Notifications are not per-user.** A single fixed `TELEGRAM_CHAT_ID`
  receives every notification for every user's tracked companies. This
  matches the tool's current scope (one person's own use), but would need a
  `telegram_chat_id` per user (and its own opt-in/verification flow) to
  support multiple people meaningfully.
- **The `jobs` table grows without bound.** Nothing prunes old job records,
  even for postings long since filled or removed.
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
- No `DELETE /companies/{id}`.
- Several career platforms are recognised but not fetchable (see
  functional-requirements.md).
