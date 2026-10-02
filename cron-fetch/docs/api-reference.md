# API Reference — CronFetch

Practical reference for building a frontend against this API: every
endpoint's exact request body, success response, and every error shape it
can return. For how the platform-detection and background sync work
internally, see [functional-requirements.md](functional-requirements.md).

## Base URL

- Local: `http://localhost:8080`
- Deployed: `https://crontfetch.onrender.com`

## Auth

All endpoints except `POST /signup`, `POST /login`, and `GET /health`
require a header:
```
Authorization: Bearer <token>
```
The token comes from `/signup` or `/login`'s response and is a JWT valid for
**24 hours**. There's no refresh flow — when it expires, log in again. There
is currently no logout endpoint; the frontend should just discard the token
client-side.

## Conventions

- All request/response bodies are JSON; send `Content-Type: application/json`
  on any request with a body.
- **Every error response**, regardless of endpoint, has this shape:
  ```json
  { "error": "human-readable message" }
  ```
  There's no machine-readable error code field — match on the HTTP status
  code and/or the message text if you need to branch behavior.
- Timestamps are ISO 8601 (`"2026-10-01T06:58:53.045582+05:30"`).

## Data shapes

These appear nested in several responses below.

**User**
```json
{
  "id": "8527e8fc-ee1f-4995-9cad-01257ff8bed1",
  "name": "Karthik",
  "email": "karthik@example.com",
  "created_at": "2026-10-01T06:58:53.04Z",
  "updated_at": "2026-10-01T06:58:53.04Z"
}
```

**Company**
```json
{
  "id": "623ea8eb-28d1-4093-8a8a-06ae405e872f",
  "name": "Linear",
  "career_url": "https://jobs.ashbyhq.com/linear",
  "platform": "ashby",
  "board": "linear",
  "min_experience_years": 8,
  "max_experience_years": null,
  "roles": ["engineer"],
  "locations": [],
  "created_at": "2026-10-01T06:58:53.04Z",
  "updated_at": "2026-10-01T06:58:53.04Z"
}
```
- `platform`/`board` are set automatically on create if detectable from
  `career_url` (and will be `""` if not detected — not an error, just means
  the jobs endpoint will 422 later; see below).
- `min_experience_years`/`max_experience_years` are `null` when unset — an
  open range on that side, not zero.
- `roles`/`locations` are `[]` when unset, meaning "no constraint" (every
  job matches), not "matches nothing."

**Job** (only appears inside a jobs response, never has its own endpoint)
```json
{
  "id": "d3bc1ced-3ce4-4086-a050-555055dbb1ff",
  "title": "Senior / Staff Fullstack Engineer",
  "location": "Europe; Remote",
  "url": "https://jobs.ashbyhq.com/linear/d3bc1ced-3ce4-4086-a050-555055dbb1ff",
  "experience": "5+ years"
}
```
`experience` is omitted entirely (not even `""`) when no stated range could
be found in that listing — check for the key's absence, not an empty string.

---

## `POST /signup`

Auth: none.

**Request**
```json
{ "name": "Karthik", "email": "karthik@example.com", "password": "atleast8chars" }
```

**Success — `201`**
```json
{ "token": "eyJhbGciOi...", "user": { "...User object..." } }
```

**Errors**
| Status | Body | When |
|---|---|---|
| 400 | `{"error":"invalid request body"}` | malformed JSON |
| 400 | `{"error":"name, email and password are required"}` | any field blank |
| 400 | `{"error":"password must be at least 8 characters"}` | |
| 409 | `{"error":"email is already registered"}` | |

## `POST /login`

Auth: none.

**Request**
```json
{ "email": "karthik@example.com", "password": "atleast8chars" }
```

**Success — `200`** — same shape as signup: `{ "token": "...", "user": {...} }`

**Errors**
| Status | Body | When |
|---|---|---|
| 400 | `{"error":"invalid request body"}` | |
| 400 | `{"error":"email and password are required"}` | |
| 401 | `{"error":"invalid email or password"}` | wrong password **or** unknown email — deliberately the same message for both, so the frontend can't be used to enumerate registered emails |

## `GET /me`

Auth: required.

**Success — `200`** — a `User` object.

**Errors**
| Status | Body | When |
|---|---|---|
| 401 | `{"error":"missing or invalid authorization token"}` | missing header, bad signature, or expired |
| 404 | `{"error":"user not found"}` | token valid but the account no longer exists |

---

## `POST /companies`

Auth: required. Adds a career page to track.

**Request** — only `name` and `career_url` are required; everything else is optional.
```json
{
  "name": "Linear",
  "career_url": "https://jobs.ashbyhq.com/linear",
  "platform": "ashby",
  "board": "linear",
  "min_experience_years": 2,
  "max_experience_years": 6,
  "roles": ["software engineer", "developer"],
  "locations": ["Bangalore, India", "Remote"]
}
```
`platform`/`board`: omit both to auto-detect from `career_url`. Supported
platform values: `talentbrew`, `greenhouse`, `workday`, `lever`, `ashby`,
`smartrecruiters`, `workable`, `recruitee`, `beesite`. Most platforms need
`board` (the company's slug on that platform) unless `career_url` itself is
a direct link on that platform (e.g. `boards.greenhouse.io/<board>`).
TalentBrew and Workday never need `board`. `beesite`'s `board` must be an
`https://` API URL, not a slug.

**Success — `201`** — the created `Company` object.

**Errors**
| Status | Body | When |
|---|---|---|
| 400 | `{"error":"invalid request body"}` | malformed JSON |
| 400 | `{"error":"name and career_url are required"}` | |
| 400 | `{"error":"career_url must start with http:// or https://"}` | |
| 400 | `{"error":"platform must be one of: talentbrew, greenhouse, workday, lever, ashby, smartrecruiters, workable, recruitee, beesite"}` | unknown value sent |
| 400 | `{"error":"career_url must be a *.myworkdayjobs.com link for workday"}` | |
| 400 | `{"error":"platform is required when board is set"}` | |
| 400 | `{"error":"board must be the BeeSite API address, e.g. https://jobs.api.example.com"}` | |
| 400 | `{"error":"board is required for <platform> unless career_url is that platform's own job board link"}` | |
| 400 | `{"error":"min_experience_years cannot be negative"}` | |
| 400 | `{"error":"max_experience_years cannot be negative"}` | |
| 400 | `{"error":"min_experience_years cannot be greater than max_experience_years"}` | |
| 401 | `{"error":"missing or invalid authorization token"}` | |
| 409 | `{"error":"you're already tracking this career page"}` | same `career_url` already saved by this user |

## `GET /companies`

Auth: required. Returns the caller's own companies, newest first.

**Success — `200`**
```json
[ { "...Company..." }, { "...Company..." } ]
```
`[]` (empty array, not `null`) if the user has none yet. No pagination —
all of them come back in one response.

**Errors:** only `401` (missing/invalid token).

## `PATCH /companies/{id}`

Auth: required. Partial update — send only the fields you want to change.
Runs the exact same validation as create (see the table above; all those
400 messages can also happen here).

**Request** — any subset of the create fields, e.g.:
```json
{ "min_experience_years": 5 }
```
Behavior specific to update:
- Omitted fields are left unchanged.
- `roles`/`locations`: sending `[]` clears it; sending the field at all
  **replaces** the whole list (never merges with the existing one).
- `null` for `min_experience_years`/`max_experience_years` clears that
  bound; omitting the field leaves it as-is.
- Sending `career_url` without also sending `platform`/`board` re-runs
  platform auto-detection against the new URL and overwrites both.
- Unknown JSON keys in the body are rejected outright (see 400 below) —
  useful for catching a typo'd field name on the frontend during development.

**Success — `200`** — the updated `Company` object.

**Errors** (in addition to the create-validation table)
| Status | Body | When |
|---|---|---|
| 400 | `{"error":"invalid request body: json: unknown field \"...\""}` | a key not in the Company shape |
| 400 | `{"error":"no fields to update"}` | empty body `{}` |
| 401 | `{"error":"missing or invalid authorization token"}` | |
| 404 | `{"error":"company not found"}` | wrong id, malformed id, or belongs to another user — same response for all three, so this can't be used to probe which ids exist |
| 409 | `{"error":"you're already tracking this career page"}` | the new `career_url` collides with another company this user already has |

**There is no `DELETE /companies/{id}` yet.**

## `GET /companies/{id}/jobs`

Auth: required. Live-fetches the company's current listings and returns
only the ones matching its saved filters. **Can take several seconds** —
TalentBrew and Workday companies especially (observed 8–17s); build the
frontend to show a loading state, not expect an instant response.

**Success — `200`**
```json
{
  "company": "Linear",
  "total_scanned": 30,
  "total_matched": 9,
  "warnings": [
    "experience filters are applied on a best-effort basis: a job whose listing doesn't clearly state a required range is included rather than excluded"
  ],
  "jobs": [ { "...Job..." } ]
}
```
- `warnings` is omitted entirely when there's nothing to say (no experience
  filter set). When present, possible strings are:
  - `"experience filters are applied on a best-effort basis: ..."` — filter
    is active; platform supports reading experience from descriptions
    (Greenhouse, Lever, Ashby, Workable, Recruitee).
  - `"experience filters are saved but not applied for this platform — its job listings don't include enough detail to check"` — filter is set, but the
    platform (Workday, SmartRecruiters, TalentBrew, BeeSite) can't support
    it; the experience filter is silently ignored for this company's results.
- `jobs` is `[]` if nothing currently matches — not an error.

**Errors**
| Status | Body | When |
|---|---|---|
| 401 | `{"error":"missing or invalid authorization token"}` | |
| 404 | `{"error":"company not found"}` | |
| 422 | `{"error":"this career site uses <platform>, which isn't supported yet — supported: ..."}` | site recognized but not fetchable (e.g. Taleo, iCIMS) |
| 422 | `{"error":"couldn't recognise this career site's platform — supported: ..."}` | nothing recognized at all |
| 422 | `{"error":"no workday career site found at the saved career_url"}` | |
| 422 | `{"error":"this site's job board name couldn't be worked out — save the company with its platform and board name"}` | |
| 422 | `{"error":"no job board found with the saved board name"}` | wrong `board` value |
| 502 | `{"error":"could not fetch jobs from the career page"}` | the career site itself errored or timed out |

---

## `GET /health`

Auth: none. For monitoring/uptime checks, not really a frontend-facing endpoint.

**Success — `200`**
```json
{ "status": "online", "db": "online" }
```
`db` can be `"unreachable"` while `status` stays `"online"` and the HTTP
status is still `200` — check the `db` field itself, not just the status code.

---

## Things the frontend should know that aren't in any single response

- **No real-time push.** New-job notifications are sent to a fixed Telegram
  chat server-side, not to the frontend. There's no websocket/SSE endpoint —
  if you want "new job" UI state, you'd poll `GET /companies/{id}/jobs` and
  diff client-side.
- **No pagination anywhere** (`GET /companies`, the `jobs` array). Fine at
  current scale; will need addressing if a user tracks many companies or a
  company has very long job lists.
- **Rate limiting:** none exists yet, on any endpoint including `/signup`/`/login`.
