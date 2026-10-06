# CronFetch — Execution Plan

Written for: you (the product owner) and anyone who will build this next.
Status: **planning only. No code has been written for the new features yet.**

This plan covers the new user-facing app (post-login screens, nearby job
alerts, notifications, profile) and how it connects to the existing backend.
Existing behaviour is documented in
[cron-fetch/docs/functional-requirements.md](../cron-fetch/docs/functional-requirements.md)
and [cron-fetch/docs/non-functional-requirements.md](../cron-fetch/docs/non-functional-requirements.md).

---

## 1. Product summary

After login the user gets:

| Screen | What it does |
|---|---|
| **Dashboard** | Lists the companies the user tracks. |
| **Search Jobs** | Manual keyword search across jobs from tracked companies, with filters. |
| **Jobs around me** | Shows **all companies in our directory** within 10 km of the user's current location. Has the **Go Online / Offline** toggle on the same page. |
| **Notifications** | Inbox of alerts. Missed alerts are kept for 5 days, then deleted. |
| **Profile** | Personal details and the job filter (see §2). |
| **Role-based views** | Chosen at login. See the roles section below. |

### Roles (decided)

| Role | Who | How it's assigned | What they see |
|---|---|---|---|
| **Seeker** | Person looking for a job | Chosen at signup | Dashboard, Search Jobs, Jobs around me, Notifications, Profile |
| **Hirer** | Company posting openings | Chosen at signup, then verified | Their posted jobs, applicants, Post a job |
| **Admin / Developer** | You and your team (one role) | Assigned by you, never self-selected | Everything, plus the directory editor, sync logs, and fetch errors |

Notes:
- Roles are stored on `users.role`. The login response returns it, and the
  frontend picks the menu from it. Routes check the role on the server, not
  only in the browser.
- A hirer posting jobs is a new feature, not a view on existing data. It
  needs its own tables (see below) and is planned as a separate phase.

**Go Online** means: the installed app keeps tracking location and sends push
notifications when a nearby company has a job that matches the user's filter.
**Go Offline** stops tracking and notifications. The page itself always works
as a snapshot.

---

## 2. Matching rules (the core logic)

Two location concepts. Keep them separate in code and in the database.

- **Tracking location**: where the user physically is (GPS). Decides *which
  companies are nearby*.
- **Filter location**: where the user wants to work (e.g. `Bangalore`,
  `Remote`). Decides *which jobs are interesting*.

### User filter (profile)

| Field | Type | Example |
|---|---|---|
| `locations` | list of text | `["Bangalore", "Remote"]` |
| `roles` | list of free text | `["react developer", "full stack developer", "PMO"]` |
| `interests` | list of free text | `["react", "data engineering"]` |
| `min_experience_years` / `max_experience_years` | int, optional | `2` / `6` |

Rules:
1. At least one field must be set. Otherwise the filter matches nothing.
2. Within the filter, **one matching field is enough.** A job that matches
   `roles` but not `locations` still counts.
3. Free text uses **keyword matching**: split the user's text into words,
   ignore case and filler words, and check the job title and description.
   A small synonym list covers cases like `PMO` ↔ `Project Management Office`
   and `HR` ↔ `Human Resources`. The list grows over time.

### Notification rule

A notification is sent only when **all** of these hold:

1. The user is **online**.
2. A **company in the directory** has an office within **10 km** of the user's
   last reported position. Any company counts, whether or not the user tracks it.
3. That company has a **currently open job** that matches the user's filter
   (rules above). The job's location must fit the filter location
   (`Bangalore` jobs for a Bangalore filter; remote jobs for a Remote filter).
4. The user has not already been alerted about this company **today**, unless
   a *new* matching job has appeared since.

Companies with no matching job never notify, so "all companies around me" on
the Jobs around me page can show companies that have nothing for this user.
That's intended: the page shows what's nearby, and notifications show what fits.

---

## 3. What already exists (inputs from the current backend)

Things we can reuse:

- **Job fetching for 9 platforms** (Greenhouse, Lever, Workday, etc.) in
  `internal/fetcher`. This is the job data source, and it's the reliable
  alternative to LinkedIn, which has no usable public job-search API.
- **A background sync loop** (`internal/jobsync`, every `FETCH_INTERVAL_HOURS`)
  that already records every job seen per company and detects new ones.
  Reuse it instead of writing a second one.
- **A `jobs` table** of every job ever seen. We build on it for "currently
  open" and "new" detection.
- **Telegram notifications** for a single fixed chat. Keep this as an
  admin/debug channel. It is not per-user.
- **JWT auth** and `GET /me`.

Things that get in the way (must be fixed in the plan):

| Gap | Why it matters | Fix |
|---|---|---|
| No CORS middleware | The browser app can't call the API. | Add CORS middleware with an allowed-origin list from env. **First task.** |
| Notifications are single-chat only | Each user needs their own alerts. | Add per-user push subscriptions and a per-user notification table. |
| Company filters are company-level | Filters now belong to the user, not each company. | Keep company `roles`/`locations` for tracking. Add a separate user profile filter. |
| Companies belong to one user | Nearby alerts need companies that *any* user might be near, not just tracked ones. | Split into a **shared company directory** (global, with coordinates and career page) and **per-user tracking** (which companies a user follows, with their own filter). |
| No coordinates on companies | Needed for the 10 km check. | Add `lat`, `lng`, `place_id`, `geocoded_at`, and an office location field to directory companies. |
| No source for the directory | Without it, "all companies around me" is empty. | Build the directory in Phase 4 (see §8 and §9 decision 5). |
| Experience filter not applied | Matching on experience isn't possible yet. | Known gap. Phase 5 option (see §8). |
| `jobs` table grows without limit | Slower queries over time. | Add pruning of closed jobs. |
| Sync is serial, single process | Notifications could duplicate if the API is scaled out. | Fine for one user now. Revisit if we scale. |
| No rate limiting, no SSRF guard | Needed before public use. | Add before any public launch (see §7). |

---

## 4. Architecture

```
┌──────────────────────────────┐        HTTPS         ┌────────────────────────────────┐
│  crontfetch-web (PWA)         │ ───────────────────► │  cron-fetch (Go API)            │
│  React + Vite + Redux         │ ◄─────────────────── │  net/http, pgx                  │
│                               │                      │                                 │
│  • Dashboard                  │                      │  existing:                      │
│  • Search Jobs                │                      │   auth, companies, jobs, sync   │
│  • Jobs around me + toggle    │                      │                                 │
│  • Notifications              │                      │  new:                           │
│  • Profile                    │                      │   profile (user filter)         │
│  • Service worker (push)      │                      │   presence (online + location)  │
│  • Geolocation API            │                      │   geo (distance, geocoding)     │
└──────────────┬───────────────┘                      │   notify (inbox, dedupe, push)  │
               │ push subscription                     └───────┬───────────────┬────────┘
               ▼                                               │               │
   ┌──────────────────────┐                                    ▼               ▼
   │ Browser push service  │ ◄──── Web Push (VAPID) ──── PostgreSQL     Google Geocoding API
   │ (FCM / Apple)         │                              (+ new tables)   (server-side key)
   └──────────────────────┘
```

### Frontend (crontfetch-web)

- Keep the current stack: React, Vite, Redux Toolkit, React Router.
- **PWA**: add `vite-plugin-pwa` for the manifest and service worker.
- **Location**: browser Geolocation API (`watchPosition` while online).
- **Map**: decision needed (see §9). Options:
  - Google Maps JavaScript API. Consistent with geocoding, but billed per map load.
  - Leaflet with OpenStreetMap tiles. Free, but tile usage has a policy.
- **Push**: service worker subscribes with the VAPID public key and sends the
  subscription to the backend.
- **Styling**: reuse the existing design tokens in `src/styles/variables.css`
  and the components in `components.css`. Add new CSS files only where the
  existing ones don't cover a screen.

### Backend (cron-fetch)

New packages, following the existing `internal/` layout:

| Package | Responsibility |
|---|---|
| `internal/profile` | Read and write the user's filter, and `role`. |
| `internal/presence` | Online/offline state and the **latest** reported position only. Auto-expires if the app stops reporting. |
| `internal/geo` | Haversine distance, nearby-company query, and the Google Geocoding client. |
| `internal/notify` | Notification inbox (5-day retention), dedupe per company per day, and sending Web Push. |
| `internal/match` | Filter-vs-job matching (keywords, synonyms, location). Pure functions, easy to test. |

Changed:

- `internal/jobsync`: after each sync, run matching for online users near
  the affected companies. Keep the Telegram message for the admin chat.
- `cmd/api/main.go`: wire CORS, the new routes, and the VAPID config.
- `internal/config`: add `GOOGLE_MAPS_API_KEY`, `VAPID_PUBLIC_KEY`,
  `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`, `ALLOWED_ORIGINS`,
  `NEARBY_RADIUS_KM` (default 10), `NOTIFICATION_TTL_DAYS` (default 5).

### Data model (new and changed)

Migrations go in `cron-fetch/migrations/`, numbered in order, as the project
already does. No migration tool yet.

```
users                 + role TEXT NOT NULL DEFAULT 'user'           -- for later role-based views

profiles              user_id PK→users, locations TEXT[], roles TEXT[],
                      interests TEXT[], min_exp INT, max_exp INT, updated_at

directory_companies   id PK, name, career_url UNIQUE, platform, board,
                      office_address TEXT, lat DOUBLE PRECISION, lng DOUBLE PRECISION,
                      place_id TEXT, geocoded_at TIMESTAMPTZ, source TEXT,  -- shared, not per user

tracked_companies     user_id→users, directory_company_id→directory_companies,
                      roles TEXT[], locations TEXT[], min_exp INT, max_exp INT, created_at
                      PRIMARY KEY(user_id, directory_company_id)

companies             -- existing table: kept for now, migrated into directory_companies +
                      -- tracked_companies in Phase 1 so the current API keeps working

presence              user_id PK→users, online BOOLEAN, lat, lng,
                      reported_at TIMESTAMPTZ          -- latest only, no history

push_subscriptions    id PK, user_id→users, endpoint UNIQUE, p256dh, auth, created_at

notifications         id PK, user_id→users, company_id→companies, job_id,
                      title, body, url, read_at, created_at, expires_at (= created_at + 5 days)

notified_companies    user_id, company_id, day DATE, PRIMARY KEY(user_id, company_id, day)
                      -- enforces "one per company per day"

jobs                  + last_seen_at TIMESTAMPTZ, closed_at TIMESTAMPTZ   -- "currently open" detection

hirer_companies       user_id→users (role=hirer), directory_company_id→directory_companies NULL,
                      name, verified BOOLEAN       -- a hirer links to a directory company or creates one

posted_jobs           id PK, hirer_company_id, title, description, location, remote BOOLEAN,
                      min_exp INT, max_exp INT, apply_url TEXT NULL, status (open|closed),
                      created_at, closed_at

```

Privacy design for location:
- The server stores only the **latest** position per user, in `presence`.
- Going offline (or no update for ~10 minutes) clears it.
- No location history is kept. Say so in the app's privacy text.

---

## 5. Key flows

### A. Go Online
1. User taps **Go Online**. The app asks for location permission.
2. `POST /presence` with `{online: true}`, then the app sends its position
   every few minutes while the app is open or running as a PWA.
3. Backend stores the latest position and sets `online = true`.

### B. Notification check
Runs in two places, both calling the same `notify.Evaluate` function:
- **On each location report** from an online user: find tracked companies
  within 10 km, then check their open jobs against the user's filter.
- **After each background sync**: for each online user near a company that
  got new jobs, check those jobs.

For each match not already sent today:
1. Insert a row in `notifications` (expires in 5 days).
2. Insert a row in `notified_companies`.
3. Send Web Push to each of the user's subscriptions.

### B2. Live company check (targeted, dynamic)
This is what happens when you pass a company, for example Anthropic on your
route. The check targets that company's own career page, not a broad sync.

1. Your reported position enters the 10 km radius of a directory company.
2. If that company's jobs were fetched less than **30 minutes** ago, use the
   cached result. Otherwise fetch its career page now, using its saved
   `platform` and `board`, through the existing fetcher.
3. Match the company's open jobs against the user's filter (§2).
4. Notify, following the dedupe rule.

**Speed.** Fast platforms (Greenhouse, Lever, Ashby) answer in about a second.
TalentBrew and Workday can take 8–17 seconds (see the NFR doc). So:
- **Prefetch** companies within a wider **15 km** ring, so the data is
  ready before you reach the 10 km line.
- The notification is sent **when the result is ready**, not held until a
  request returns.

**Sharing and limits.**
- Cache results per company, shared across users. Ten users near Anthropic
  cause one fetch, not ten.
- One fetch per company at a time, with a cooldown per company.
- A global limit on concurrent fetches, so the server doesn't flood a career site.
- A company with no supported platform has no live check. The directory marks
  it, and it only appears in Jobs around me.

**Caveat.** This only works for companies whose career page we can fetch.
Each directory entry needs a working `platform`/`board`, which the existing
detection already provides for most sites.

### C. Missed notifications
When the user opens the app, the **Notifications** page lists unread items
from the last 5 days. A daily job deletes rows past `expires_at`.

### D. Jobs around me (snapshot)
1. Read the current position (one-time).
2. `GET /nearby` returns directory companies within 10 km. Each entry shows
   the company's open jobs that match the user's filter, if any. This works
   whether online or offline.

### E. Search Jobs
`GET /jobs/search?q=&location=&...` searches the `jobs` table for open jobs
from tracked companies. Filters are the same as the profile filter. No live
site fetch on each search.

---

## 6. API additions (planned)

| Method | Path | Purpose |
|---|---|---|
| GET | `/profile` | Get the user's filter. |
| PUT | `/profile` | Replace the user's filter. |
| POST | `/presence` | `{online, lat?, lng?}`. Go online, report position, or go offline. |
| GET | `/presence` | Current online state (for the toggle). |
| GET | `/nearby` | Tracked companies within 10 km, with matching open jobs. |
| GET | `/jobs/search` | Search open jobs from tracked companies. |
| GET | `/notifications` | Inbox for the last 5 days. |
| POST | `/notifications/{id}/read` | Mark as read. |
| POST | `/push/subscribe` | Save the browser's push subscription. |
| DELETE | `/push/subscribe` | Remove it. |
| GET | `/config/push` | Return the VAPID public key to the frontend. |

Each route follows the existing conventions: JWT auth, scoped to the caller,
the same error shape, and documented in `functional-requirements.md` when
built.

---

## 7. Security, privacy and reliability

Before any public use:
- **HTTPS everywhere.** Geolocation and push both require a secure origin
  (localhost is the only exception, for development).
- **CORS** with an explicit origin list. No `*` with credentials.
- **Rate limits** on `/login`, `/signup`, `/presence`, and `/nearby`.
- **SSRF guard** on the career URL fetcher (block private IP ranges). Already
  listed as a known gap.
- **Location consent**: ask once, explain what is stored (latest point only),
  and make Go Offline obvious.
- **Push subscriptions** are per-user and deleted on logout.
- **Geocoding key** stays server-side. The browser map key is restricted to
  our domains.
- **Retries** for failed push sends, with a limit. Currently the sync has no
  retry.
- **Transaction** around "record notification" and "send push", so a crash
  doesn't leave an alert recorded but never sent.

---

## 8. Phased build plan

Sizes are relative (S/M/L), not dates. Each phase ends in something you can
run and check.

### Phase 0 — Prerequisites (no feature code)
- [ ] Decide the open questions in §9.
- [ ] Google Cloud project with billing, Geocoding API enabled, server key
      and a restricted browser key. (You create these.)
- [ ] VAPID key pair, generated by us and stored in `.env`.
- [ ] Confirm which Postgres the migrations run against.
- Size: S

### Phase 1 — Backend foundation
- [ ] CORS middleware, with `ALLOWED_ORIGINS` from env.
- [ ] `users.role` column (default `user`).
- [ ] `profiles` table and `GET/PUT /profile`.
- [ ] `companies` coordinate columns and office address field.
- [ ] `jobs.last_seen_at` / `closed_at`, set by the existing sync.
- [ ] Tests: CORS, profile validation (at least one field set).
- Size: M

### Phase 2 — Frontend screens with mock data
- [ ] Routes and nav for Dashboard, Search Jobs, Jobs around me,
      Notifications, Profile.
- [ ] Dashboard: replace the empty-state placeholder with a company card list.
- [ ] Profile: the filter form (location, roles, interests, experience).
- [ ] Search Jobs: keyword and filter UI on mock results.
- [ ] Jobs around me: map placeholder, company list, and the Go Online toggle
      (UI only).
- [ ] Notifications: list with read state, showing a 5-day label.
- [ ] Mock data isolated in one module so swapping to real API calls is one
      change per screen.
- Size: L

### Phase 3 — Connect the screens to the real API
- [ ] Wire Dashboard, Profile, Search Jobs to the real endpoints.
- [ ] Verify CORS in the browser.
- [ ] Update `functional-requirements.md` for each new route.
- Size: M

### Phase 4 — Company directory, location, geocoding and nearby
- [ ] Create `directory_companies` and `tracked_companies`. Move the current
      per-user companies into them (Phase 1 migration).
- [ ] Build the directory. Sources to decide in §9 decision 5. Each entry needs
      a name, office address, and a career page we can fetch.
- [ ] Geocode office addresses on save (server-side), store `place_id` and
      coordinates.
- [ ] Geocoding backfill for the directory.
- [ ] `presence` table, `POST /presence`, expiry of stale positions.
- [ ] `GET /nearby` over the directory, with haversine distance and the 10 km
      radius.
- [ ] Jobs around me: real position, real companies, real map.
- [ ] Tests: distance calculation, radius edges, expiry.
- Size: L

### Phase 5 — Matching and notifications
- [ ] `internal/match`: keyword matching, synonym list, location rules.
- [ ] `notify.Evaluate` wired into the sync and into location reports.
- [ ] `notifications` and `notified_companies` tables, 5-day expiry job.
- [ ] Web Push: service worker, subscription, sending from the backend.
- [ ] Notifications page reads from the inbox.
- [ ] Tests: matching rules (each one, and the combinations), dedupe per day,
      new-job-overrides-dedupe, offline users get nothing.
- Size: L

### Phase 6 — PWA install and device testing
- [ ] Manifest, icons, install prompt.
- [ ] Test on Android Chrome with the app installed, phone locked and moving.
- [ ] Test on iPhone (iOS 16.4+) installed to the home screen. Expect weaker
      background behaviour; document what works.
- [ ] Battery check: measure location-update cost over a 1-hour drive.
- Size: M

### Phase 7 — Hardening
- [ ] Rate limits, SSRF guard, HTTPS deployment.
- [ ] Pruning of closed jobs.
- [ ] Retry and transaction handling for notifications.
- [ ] Update `non-functional-requirements.md`.
- Size: M

### Phase 7b — Roles and hiring (planned)
- [ ] `users.role` set at signup (seeker or hirer). Admin/developer assigned
      manually.
- [ ] Role check on every route. Login returns the role.
- [ ] Hirer: create company profile, verification, post, edit and close jobs.
- [ ] Seeker: Search Jobs includes hirer-posted jobs, not only scraped ones.
- [ ] Apply button opens the job's career page in a new tab. No application tracking.
- Size: L (its own planning pass before building)

### Later (not scheduled)
- Admin/developer tools beyond the directory editor (sync logs, fetch errors).
- Experience filter applied to matching. Needs each shortlisted job's
  full description fetched and parsed for a stated range.
- Aggregator API (e.g. Adzuna or JSearch) for wider search beyond tracked
  companies, after checking terms and India coverage.
- Native mobile app, if PWA background behaviour on iPhone isn't good enough.

---

## 9. Decisions still needed

These affect the build. Please answer before Phase 1 starts where marked.

**Decided:**
- **Nearby companies = all companies in the directory**, notify only when the
  user's filter matches. *(Decision 1)*
- **Office location is geocoded** from the address, separate from the
  filter's location text. *(Decision 2 and 3)*
- **Map display: Leaflet + OpenStreetMap. Google is used only for geocoding.**
  *(Decision 4)*

**Decided:**
- **Location matching is text-based at city level.** A job whose location
  contains the filter's city matches it (`Bangalore` filter matches a job
  listed as `Bangalore`; `Delhi` matches `Delhi`). No neighbourhood list.
  Synonyms such as `Bengaluru` ↔ `Bangalore` go in the synonym list.

**Decided:**
- **Apply opens the company's career page** (the job's `url` or `apply_url`).
  CronFetch doesn't store applications, so there's no `applications` table.
  *(Decision 0)*
- **Company directory sources:**
  - **Admin** adds companies directly: name, career page URL, office address.
  - **Verified hirers** can add their own company. The entry is **pending**
    until an admin approves it, and it isn't used for nearby results until then.
  - **Duplicates** are caught by career URL and by name before saving.
  - The career page is checked when a company is saved, so unreadable links are
    caught early.
  - Hirer-posted jobs (in-app) are included in matching alongside directory jobs.
  *(Directory decision. Replaces the earlier options a/b/c.)*

**Deferred (does not block Phases 1–2):**
1. **Experience matching**: keep as a warning only until Phase 7, or build
   full-description parsing now? **Recommendation: later.**
2. **Telegram**: keep the admin channel alongside push? **Recommendation: yes.**

---

## 10. Testing approach

| Layer | Tool | What to cover |
|---|---|---|
| Go unit | `go test` | Distance, radius edges, matching rules, dedupe, expiry. |
| Go integration | `go test` + test Postgres | Profile, presence, notification inbox, sync to notification. |
| Frontend unit | Vitest + React Testing Library | Filter form validation, toggle state, notification list. |
| Frontend manual | Real devices | Install, location permission, push while app is closed. |
| Field test | Drive a route | Notifications fire near the right companies, not elsewhere. |

---

## 11. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| iPhone background location and push are weak in a PWA | Some users miss alerts | Document it; plan the native-app option. |
| Career sites change or block scraping | Matches drop silently | Per-company error logs; the existing fetcher already reports platform errors. |
| Geocoding costs and terms | Bills or policy issues | Geocode once per company, store `place_id`; read current Google terms. |
| Battery drain from location | Users turn it off | Low-frequency updates, move-triggered where possible; measure in Phase 6. |
| Keyword matching is too loose or too tight | Wrong or missing alerts | Start with the simple rule, log near-misses, grow the synonym list. |
| Single-process sync and notifications | Duplicates if scaled | Accept for now; add a lock if we run more than one instance. |

---

## 12. What I need from you to start

1. Answer §9 open question 1 (where the directory comes from). Phase 4 can't
   start without it. Question 2 is needed before Phase 5.
2. Google Cloud project with billing and a Geocoding key, when we reach Phase 4.
3. Database access for running migrations (or confirm the local setup in the README).
4. A hosting plan with HTTPS, needed before Phase 6 (and for location on phones).
5. Go-ahead to start Phase 1 (backend foundation) and Phase 2 (frontend mock screens).

Nothing in this document has been built yet.
