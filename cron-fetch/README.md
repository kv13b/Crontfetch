# CronFetch — backend

A small Flask REST API that watches job-listing pages on a schedule and records
only the postings that match your experience level.

You register a site by **URL only** and tell it how many years of experience you
have. CronFetch fetches the page, auto-detects the CSS selector for the job
cards, and from then on — every 15 minutes — pulls out the individual listings,
reads the "years of experience" requirement from each one, and logs the **new**
listings that fit. Listings that don't match, or that don't state an experience
requirement, are ignored.

This repo is the backend only. A React dashboard and a Telegram notifier are
planned to sit in front of it.

## How it works

```
React frontend  ──HTTP──▶  Flask API  ──▶  SQLite (instance/cronfetch.db)
                                │
                                └─ APScheduler: run_check() every 15 min
                                     ├─ fetch each site (requests)
                                     ├─ auto-detect job-card selector (first time)
                                     ├─ extract job cards (BeautifulSoup)
                                     ├─ parse experience from card text
                                     └─ store new matches → jobs + history
```

- **`app.py`** – app factory, CORS, `/health`, starts the background scheduler.
- **`database.py`** – SQLite connection helper (`sqlite3.Row`), schema, lightweight
  `ALTER TABLE` migrations.
- **`scraper.py`** – `run_check()`: the fetch → detect → extract → filter → store
  pipeline; `probe_site(url)` for on-demand detection.
- **`detect.py`** – `detect_listings(html, url)`: finds the repeating job-card
  selector automatically (known-ATS patterns + a generic scoring heuristic).
- **`experience.py`** – `parse_experience(text)` and `experience_matches(...)`.
- **`routes/`** – one Blueprint per resource (`sites`, `jobs`, `history`).
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
`entry level`. It returns `(min, max)` (with `max = None` for open-ended) or
`None` when nothing is found.

A listing is kept when `job_min ≤ your_years ≤ job_max`. `None` (unparseable)
is skipped. Set a site's `min_experience` to `null` to disable filtering for
that site.

### First-run seeding

The first time a site is checked, listings already on the page are stored
silently (no notifications) so you don't get flooded when adding an established
board. Alerts begin from the second check onward.

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
| `POST` | `/api/sites` | `{name, url, min_experience?, item_selector?}` | Add a site; auto-detects the selector and returns a `detection` object |
| `POST` | `/api/sites/<id>/detect` | – | Re-run auto-detection and save the result |
| `PATCH` | `/api/sites/<id>` | any subset of `name, url, css_selector, item_selector, min_experience` | Edit a site (change the filter, fix the URL, set a manual selector) |
| `DELETE` | `/api/sites/<id>` | – | Remove a site (its jobs cascade) |
| `GET` | `/api/jobs` | – | Last 100 matched jobs, with `site_name` |
| `GET` | `/api/history` | – | Last 200 check/notification log rows |
| `POST` | `/api/run-now` | – | Run `run_check()` immediately |

`min_experience` is your years of experience as a whole number, or `null` for no
filter. `item_selector` is optional and overrides auto-detection;
`css_selector` is a last-resort fallback used only when neither an auto-detected
nor a manual `item_selector` exists.

### Example

```bash
curl -X POST http://127.0.0.1:5000/api/sites \
  -H "Content-Type: application/json" \
  -d '{"name":"Acme Careers","url":"https://acme.example/careers","min_experience":3}'
# -> 201 { ..., "item_selector": "li.job-card", "selector_source": "auto",
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
├── routes/
│   ├── __init__.py     # exposes `blueprints`
│   ├── sites.py
│   ├── jobs.py
│   └── history.py
├── instance/           # SQLite file lives here (git-ignored)
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
python experience.py          # runs the experience parser against sample titles
```

### Updating dependencies

```bash
pip install <package>
pip freeze > requirements.txt
```

### Notes / not done yet

- No auth — bind to localhost or put it behind a reverse proxy.
- Dev server only; use a WSGI server (gunicorn/waitress) for anything real.
- Telegram notifications: `history` rows are written with `status="pending"` but
  nothing sends them yet.
- A site with no usable selector (JS-rendered / bad URL) logs a `history` error
  on every scheduled run until fixed — no back-off yet.
- Experience is parsed from the **listing card** text only; sites that show years
  of experience only on the job detail page won't filter (all their listings
  count as "experience unknown" and are skipped unless `min_experience` is null).
- No automated test suite yet.
