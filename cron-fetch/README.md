# CronFetch — backend

A small Flask REST API that watches job-listing pages on a schedule and records
only the postings that match your experience band, preferred locations, and role.

You register a site by **URL only** and set independent filters: an experience
band (`min_experience`/`max_experience`), which locations you want (e.g.
`India`, `Remote` — the frontend offers a fixed list, you pick any number of
them), and which roles you want (e.g. `Engineering`, `Software Developer`).
CronFetch fetches the page, auto-detects the CSS selector for the job cards,
and from then on — every 15 minutes — pulls out the individual listings, reads
the experience requirement, location, and role from each one's text, and logs
the **new** listings where **every filter you set** passes, and pushes each one
to Telegram. Listings that fail any one of them, or that don't state an
experience requirement when an experience filter is set, are ignored.

This repo is the backend only. A React dashboard is planned to sit in front of it.

## How it works

```
React frontend  ──HTTP──▶  Flask API  ──▶  SQLite (instance/cronfetch.db)
                                │
                                └─ APScheduler: run_check() every 15 min
                                     ├─ fetch each site (requests)
                                     ├─ auto-detect job-card selector (first time)
                                     ├─ extract job cards (BeautifulSoup)
                                     ├─ parse experience + location + role from card text
                                     ├─ store new matches → jobs + history (status=pending)
                                     └─ notifier.send_pending() → Telegram (status=sent)
```

- **`app.py`** – app factory, CORS, `/health`, starts the background scheduler.
- **`database.py`** – SQLite connection helper (`sqlite3.Row`), schema, lightweight
  `ALTER TABLE` migrations.
- **`scraper.py`** – `run_check()`: the fetch → detect → extract → filter → store
  pipeline; `probe_site(url)` for on-demand detection.
- **`detect.py`** – `detect_listings(html, url)`: finds the repeating job-card
  selector automatically (known-ATS patterns + a generic scoring heuristic).
- **`experience.py`** – `parse_experience(text)` and `experience_matches(...)`.
- **`location.py`** – `parse_locations(raw)` and `find_location(locations, text)`.
- **`role.py`** – `parse_roles(raw)` and `find_role(roles, text)` (same pattern
  as `location.py`, applied to job title/department instead of location).
- **`notifier.py`** – `send_pending(conn)`: sends every unsent match to
  Telegram and marks it `sent`; a no-op when Telegram isn't configured.
- **`routes/`** – one Blueprint per resource (`sites`, `jobs`, `history`, `notify`).
- **`instance/`** – holds the SQLite file (git-ignored, created on first run).

### Selector auto-detection

You don't supply a CSS selector. On the first fetch of a site (and on
`POST /api/sites/<id>/detect`), `detect.py`:

1. checks the host against known ATS patterns (Greenhouse, Lever, SmartRecruiters,
   Workable, BambooHR);
2. otherwise groups every link's ancestor elements by structural signature and
   scores each group on: how many similar siblings, whether each has its own
   link, job-ish URLs (`/job/`, `/careers/…`), a shared URL path, and role words
   in the text;
3. returns the best selector, or `null` with a note (e.g. *"page appears to be
   JavaScript-rendered"*) when nothing scores confidently.

The detected selector is saved to `sites.item_selector` with
`selector_source = 'auto'`. Passing `item_selector` in the API sets it to
`'manual'` and auto-detection leaves it alone. JavaScript-rendered boards
(Workday, Ashby, many custom React boards) can't be scraped statically — the
site is still created, but flagged.

### Experience matching

`parse_experience()` understands text such as `2-4 years`, `5+ years`,
`minimum 3 years`, `up to 2 years`, `7 years experience`, `fresher` /
`entry level`. It returns `(job_min, job_max)` (with `job_max = None` for
open-ended) or `None` when nothing is found.

`min_experience` and `max_experience` describe **your** band, and a listing is
kept when its own `(job_min, job_max)` **overlaps** that band:

| you set | your effective band | meaning |
|---|---|---|
| `min_experience` only | `[min, min]` | point match — a listing must be pitched at exactly that level (this is the original single-value behaviour, unchanged) |
| `max_experience` only | `[0, max]` | "nothing more senior than this" |
| both | `[min, max]` | a real range — a listing matches if it overlaps at all |
| neither | no filter | every listing passes this check |

A listing with no detectable experience (`parse_experience()` returned `None`)
is skipped whenever either bound is set. `min_experience` may not be greater
than `max_experience` — the API rejects that with 400. Set both to `null` to
disable experience filtering for a site.

### Location matching

`locations` is a list you set per site (typically chosen from a fixed set in
the frontend, e.g. `India`, `Remote`). There's no geocoding — `find_location()`
just checks, case-insensitively, whether any of those strings appears anywhere
in the listing's card text. This works because job cards embed their location
directly (`"Backend Developer  Bengaluru, Karnataka, India  Engineering"`).

A listing is kept only if it contains **at least one** of the selected
locations (OR across your choices). `matched_location` on the stored job
records which one matched. Set `locations` to `null` / `[]` to disable
location filtering for that site.

### Role matching

`roles` works exactly like `locations`, just matched against the same card
text for role/title keywords instead (e.g. `Engineering`, `Software
Developer`, `Data Science`). It's a plain substring check, not a taxonomy —
"Software Developer" won't match a card that only says "Software Engineer",
so list every phrasing you'd accept (`["Engineer", "Developer", "SDE"]`, say).
`matched_role` on the stored job records which one matched. Set `roles` to
`null` / `[]` to disable it.

### Combining filters

The experience band, `locations`, and `roles` are fully independent — a
listing is recorded only when **every filter that's set** passes. Within
`locations` or `roles` it's OR (any one entry is enough); across the three
filter types it's AND.

### First-run seeding

The first time a site is checked, listings already on the page are stored
silently (no notifications) so you don't get flooded when adding an established
board. Alerts begin from the second check onward.

### Telegram notifications

1. Message **[@BotFather](https://t.me/BotFather)** on Telegram, send `/newbot`,
   follow the prompts. You get back a token like `123456789:AAExampleTokenHere`.
2. Send any message to your new bot, then open
   `https://api.telegram.org/bot<TOKEN>/getUpdates` in a browser and read the
   `"chat":{"id": ...}` value — that's your chat id (a group works too, and its
   id is negative).
3. Copy [.env.example](.env.example) to `.env` and fill in `TELEGRAM_BOT_TOKEN`
   and `TELEGRAM_CHAT_ID`. `.env` is git-ignored; `app.py` loads it automatically
   via `python-dotenv`.
4. Restart the server, then `POST /api/notify/test` to confirm delivery before
   waiting on a real job match.

Once configured, every `run_check()` — scheduled or via `/api/run-now` — ends
by calling `notifier.send_pending()`, which sends one plain-text message per
new match (title, experience/location/role, url) and flips its `history` row
from `pending` to `sent`. If Telegram isn't configured, or a send fails (bad
token, network blip, chat not started), the row is left `pending` and picked
up automatically on the next run — nothing is lost, there's no separate retry
queue to manage. `POST /api/notify` retries immediately instead of waiting for
the next scheduled check; `GET /api/notify/status` reports whether the two env
vars are set (not whether they're valid — use the test endpoint for that).

Each `jobs` row insert now also writes its id onto the linked `history` row
(`history.job_id`), which is how `send_pending()` builds a message with the
job's actual title/url/experience/location/role instead of just the log text.

## Requirements

- Python 3.11+ (developed on 3.14)
- No external services — SQLite is file-based and bundled with Python.

## Setup

```powershell
# Windows / PowerShell
git clone <repo-url>
cd cron-fetch

py -m venv .venv
.\.venv\Scripts\Activate.ps1
pip install -r requirements.txt
```

```bash
# macOS / Linux
git clone <repo-url>
cd cron-fetch

python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

Optional: copy `.env.example` to `.env` and fill in `TELEGRAM_BOT_TOKEN` /
`TELEGRAM_CHAT_ID` (see [Telegram notifications](#telegram-notifications)
above) if you want new matches pushed to Telegram. The app runs fine without
it — matches just accumulate as `pending` in `/api/history` until you set it up.

## Running

```bash
python app.py
```

Serves on `http://127.0.0.1:5000`. The database and its tables are created
automatically on startup. The scheduler runs in-process; the first automatic
check happens 15 minutes after boot (`CHECK_INTERVAL_MINUTES` in `app.py`).

Health check:

```bash
curl http://127.0.0.1:5000/health      # {"status": "online"}
```

## API

Base URL: `http://127.0.0.1:5000`

| Method | Path | Body | Description |
|---|---|---|---|
| `GET` | `/health` | – | Liveness probe |
| `GET` | `/api/sites` | – | All watched sites |
| `POST` | `/api/sites` | `{name, url, min_experience?, max_experience?, locations?, roles?, item_selector?}` | Add a site; auto-detects the selector and returns a `detection` object |
| `POST` | `/api/sites/<id>/detect` | – | Re-run auto-detection and save the result |
| `PATCH` | `/api/sites/<id>` | any subset of `name, url, css_selector, item_selector, min_experience, max_experience, locations, roles` | Edit a site (change a filter, fix the URL, set a manual selector) |
| `DELETE` | `/api/sites/<id>` | – | Remove a site (its jobs cascade) |
| `GET` | `/api/jobs` | – | Last 100 matched jobs, with `site_name`, `matched_location`, `matched_role` |
| `GET` | `/api/history` | – | Last 200 check/notification log rows (`job_id` links a match to its job) |
| `POST` | `/api/run-now` | – | Run `run_check()` immediately, then push new matches to Telegram; response includes a `notified` summary |
| `GET` | `/api/notify/status` | – | `{"configured": bool}` — are `TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID` set |
| `POST` | `/api/notify/test` | – | Send a one-off test message; 400 if unconfigured, 502 if Telegram rejects it |
| `POST` | `/api/notify` | – | Retry any match still `pending` right now, instead of waiting for the next check |

`min_experience`/`max_experience` are whole numbers of years describing your
band (see the table above for how a single bound behaves); `min_experience`
greater than `max_experience` is rejected with 400 (checked against whatever
is already stored, so a `PATCH` that only touches one bound is validated
against the current value of the other). `locations` and `roles` are each a
list of strings (or a comma-separated string) such as `["India", "Remote"]` /
`["Engineering", "Software Developer"]`, or `null`/`[]` for no filter — every
filter that's set must pass for a listing to be recorded. `item_selector` is
optional and overrides auto-detection; `css_selector` is a last-resort
fallback used only when neither an auto-detected nor a manual `item_selector`
exists.

### Example

```bash
curl -X POST http://127.0.0.1:5000/api/sites \
  -H "Content-Type: application/json" \
  -d '{"name":"Acme Careers","url":"https://acme.example/careers","min_experience":2,"max_experience":5,"locations":["India","Remote"],"roles":["Engineering","Software Developer"]}'
# -> 201 { ..., "item_selector": "li.job-card", "selector_source": "auto",
#          "min_experience": 2, "max_experience": 5,
#          "locations": ["India", "Remote"], "roles": ["Engineering", "Software Developer"],
#          "detection": { "ok": true, "count": 25, "sample": [...], "note": "..." } }

curl -X POST http://127.0.0.1:5000/api/run-now    # 1st call seeds, later calls alert
curl http://127.0.0.1:5000/api/jobs
```

## Contributing

### Project layout

```
cron-fetch/
├── app.py              # entrypoint + scheduler
├── database.py         # connection, schema, migrations
├── scraper.py          # run_check() pipeline + probe_site()
├── detect.py           # automatic job-card selector detection
├── experience.py       # experience parsing / matching
├── location.py         # location filter parsing / matching
├── role.py             # role/title filter parsing / matching
├── notifier.py         # Telegram delivery (send_pending, send_message)
├── routes/
│   ├── __init__.py     # exposes `blueprints`
│   ├── sites.py
│   ├── jobs.py
│   ├── history.py
│   └── notify.py
├── instance/           # SQLite file lives here (git-ignored)
├── .env.example        # Telegram env var template - copy to .env
├── requirements.txt
└── README.md
```

### Conventions

- New endpoints go in a Blueprint under `routes/`. Register it by adding it to
  the `blueprints` tuple in `routes/__init__.py`.
- Get a DB handle with `database.get_connection()`; close it in a `finally`.
  One connection per request/task — don't share them across threads.
- Schema changes: update `SCHEMA` in `database.py` **and** add an idempotent
  `ALTER TABLE` entry to `MIGRATIONS` so existing databases upgrade in place.
- 4-space indent, standard library import group first. Match the style of the
  file you're editing.

### Testing the API

Import **`CronFetch.postman_collection.json`** into Postman (or Insomnia — it
reads the same format). It has one request per endpoint, grouped into folders,
with a `base_url` variable and inline docs on every field.

- Run **Sites > Create site** first; its test script saves the new id into the
  `site_id` variable that **Update site** and **Delete site** reuse.
- **Scraper > Run now** triggers a check immediately instead of waiting for the
  15-minute timer. The first call after adding a site only seeds; matches start
  alerting on the second call.

Point a site at a page you control (or a local `python -m http.server`) to
exercise the pipeline with predictable content.

```bash
python experience.py          # runs the experience parser + band matching against sample titles
python location.py            # runs the location matcher against sample titles
python role.py                # runs the role matcher against sample titles
python notifier.py            # sends a real test message if .env is configured
```

### Updating dependencies

```bash
pip install <package>
pip freeze > requirements.txt
```

### Notes / not done yet

- No auth — bind to localhost or put it behind a reverse proxy.
- Dev server only; use a WSGI server (gunicorn/waitress) for anything real.
- No `/api/settings` endpoint yet — Telegram credentials are environment
  variables, not something you can change through the API.
- A site with no usable selector (JS-rendered / bad URL) logs a `history` error
  on every scheduled run until fixed — no back-off yet.
- Experience is parsed from the **listing card** text only; sites that show years
  of experience only on the job detail page won't filter (all their listings
  count as "experience unknown" and are skipped whenever `min_experience` or
  `max_experience` is set).
- An open-ended job requirement ("1+ years", parsed as `(1, None)`) is treated
  as having no known upper bound, so it can still pass a `max_experience`
  filter as long as its stated minimum is within your band — there's no way to
  tell from the text alone whether such a posting is actually capped.
- Location matching is a plain substring check on the same card text — no
  geocoding, no city/country hierarchy (selecting "India" won't also match a
  card that only says "Bengaluru" unless the card text also says "India").
  Sites that put location only on the detail page won't filter by it either.
- Role matching is the same plain substring check — no synonym list, so
  "Software Developer" won't match "Software Engineer" on its own. Add every
  phrasing you want as a separate entry in `roles`.
- No automated test suite yet.
