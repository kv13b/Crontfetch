# Maps and Jobs around me — build log

Status as of 2026-10-31. Covers the first slice of the "online" features in
[execution-plan.md](execution-plan.md): company locations, nearby search,
real companies around the user, and the map on **Jobs around me**.

Committed on `dev`: `fb68c13` (locations, `/nearby`, first map) and
`5e8da24` (map features). The "companies around you" work below (import,
fallback sources, grey pins, Track button) is not committed yet. A visual
walkthrough for new engineers, with a diagram per flow and a file map, is in
[jobs-around-me-guide.html](jobs-around-me-guide.html) (open it in a browser).

## Goal: no paid services

The plan originally used Google Maps and Google Geocoding. Everything below
uses free OpenStreetMap (OSM) data instead:

| Need | Free option used | Limits |
|---|---|---|
| Map display | Leaflet + OpenStreetMap tiles | Attribution required (shown on the map). |
| Address → coordinates | Nominatim | Max 1 request/second, identifying User-Agent. The server enforces both. |
| Real companies around a point | OSM extract imported into our Postgres (`cmd/importplaces`) | None at request time. Re-import to refresh. |
| …fallback for areas not imported | Overpass API (4 mirrors), then Photon | Public servers, often overloaded (see below). |
| User's position | Browser Geolocation API | Works on `localhost` and HTTPS only. |

### Why an import, not live lookups

The first version asked Overpass for offices on every page load. In testing
on 2026-10-31 that wasn't reliable:

- All four public Overpass mirrors timed out or returned 502/504 for long
  stretches. When one did answer, it returned 300 offices within 5 km of
  Panathur, so the data is there.
- Photon (komoot's free search) answered in about 2 seconds and found 180
  companies within 10 km, but stopped answering after a burst of about 60
  requests during testing.

So offices are now imported once from a downloaded extract and served from
our own database. The public servers are only used for areas the import
doesn't cover.

## What was built

### Database

- `007_add_geo_and_job_status.sql`: `companies` gets `office_address`, `lat`,
  `lng`, `place_id`, `geocoded_at`; `jobs` gets `last_seen_at`, `closed_at`.
- `008_create_osm_places_table.sql`: `osm_places` (id, name, kind, website,
  address, lat, lng) with a `(lat, lng)` index.

Both are applied to the local Docker database, and `osm_places` holds the
Karnataka import.

### Backend (`cron-fetch`)

| Piece | What it does |
|---|---|
| `internal/geo/geo.go` | `DistanceKm` (haversine), `ValidCoords`, Nominatim geocoder. |
| `internal/geo/places.go` | `Places`: tries sources in order, caches public-server results for 24 h per ~5 km cell, one lookup at a time, skips a failed source for 10 min, dedupes, filters by radius, sorts by distance. |
| `internal/geo/db.go` | `DBSource`: reads `osm_places`. First in line, not cached. |
| `internal/geo/overpass.go` | `Overpass`: 4 mirrors in turn, starting with the last one that worked, 25 s budget. Also the shared OSM tag → place mapping. |
| `internal/geo/photon.go` | `Photon`: 50 nearest offices plus one search per common company-name word, 2 at a time. Gives up at once if the nearest lookup fails. |
| `cmd/importplaces` | Reads an `.osm.pbf` extract, keeps named offices (`office` = company, it, software, telecommunication, engineering, consulting, research, financial), places building outlines at their centre, replaces `osm_places` in one transaction. Optional `-bbox`. |
| `internal/handlers/location.go` | The three location routes below. |
| `internal/fetcher/careerpage.go` | `FindCareerPage`: the company's website plus name-based board lookups on six platforms; boards must have jobs, and name-reporting platforms must match the name. |
| `internal/handlers/careerpage.go` | `GET /career-page`. |
| `internal/jobsync/db.go` | A sync refreshes `last_seen_at` (and clears `closed_at`) on jobs it sees again. |

Routes (all need the JWT):

| Method | Path | Purpose |
|---|---|---|
| PUT | `/companies/{id}/location` | Set an office location: `lat`/`lng` (a map pin) or just `office_address` (looked up with Nominatim). `{}` clears it. |
| GET | `/nearby?lat=&lng=&radius_km=` | The caller's tracked companies within the radius (default 10 km, max 50), closest first. |
| GET | `/nearby/places?lat=&lng=&radius_km=` | Real offices around the point, tracked or not, with `source` (`database`, `overpass` or `photon`). |
| GET | `/career-page?name=&website=` | Suggests a career page for Track. Nothing is saved. |

Details: [cron-fetch/docs/api-reference.md](../cron-fetch/docs/api-reference.md).

New dependency: `github.com/paulmach/osm` (PBF reader), used only by the
importer. The importer builds only with `CGO_ENABLED=0`, so it uses Go's own
zlib instead of a C one; the API build is unaffected.

### Frontend (`crontfetch-web`)

- `NearbyMap.tsx`: your position, the 10 km circle, green pins for tracked
  companies, grey pins for other offices. A grey pin's popup has the name,
  distance, address, website (if known) and **Track this company**.
- `NearbyPage.tsx`: map; **Companies around you** list with a name search
  and "Show all"; **Office locations** for tracked companies (address
  lookup or pick on map). Offices you already track are hidden from the
  grey list. The OSM lookup loads separately, so a slow one doesn't hold up
  your tracked companies.
- `AddJobs.tsx`: **Track** opens Add Jobs with the name filled in, then
  looks up the career page in the background (`GET /career-page`) and fills
  it in unless you've already typed one, with a note on how it was found.
  Roles, locations and experience are copied from the Jobs around me filter
  (Max Exp = your own years; areas of interest aren't a company filter).
  After saving, the office location is set automatically and you go back to
  Jobs around me, where it shows as a green pin.
- `nearbyService.ts`: `fetchNearbyPlaces` and the `TrackPrefill` type.

## Tested

- Unit tests (all pass): distance, coordinates, Nominatim parsing; Overpass
  parsing, mirror fallback and "remember the working mirror"; Photon parsing
  and early give-up; `Places` radius filter, sorting, caching, source
  fallback, cooldown, "no data here" without cooldown, uncached database
  source; dedupe.
- Against the real API and local database (throwaway account, deleted
  afterwards): pins at 1.4 km and 5.2 km returned closest first; Nominatim
  lookup of "Koramangala, Bengaluru" returned 12.9357, 77.6241.
- Live public servers: see "Why an import" above.
- Import of `karnataka.osm.pbf` (129 MB, OpenStreetMap France, 30 Oct
  2026): 1,702 office points and 136 outlines, 1,838 offices saved, 2 seconds.
  By type: company 1,423, it 182, financial 108, telecommunication 61,
  research 44, consulting 20.
- End to end through the API near Panathur (12.935, 77.705): 874 offices
  within 10 km from the `database` source in about 15 ms (Bazaarvoice,
  Rakuten, Cisco, Walmart Labs, InMobi, Salesforce, Deloitte…); 207 of them
  have a website.
- Career page lookup on 25 random offices near Panathur (1 Nov 2026): a
  readable job board for 10 (e.g. Accenture's Workday board via
  accenture.com/in-en/careers, Altisource on SmartRecruiters, Exponent Energy
  on Workable), the website or a careers page for 5, nothing for 10. Big
  firms on their own career systems (IBM, Atlassian, Walmart) get nothing.
  A first version accepted empty Workable accounts named "IBM", "Deloitte",
  "Uber"; boards now need at least one job.
- Outside the import (Chennai): the database reported no data, Overpass and
  Photon were both down, and the API returned `502` after 40 seconds, as
  designed.
- Frontend production build passes.

## Not done yet

- **Career pages aren't always found.** In a sample of 25, 10 got a
  readable job board. Companies on unsupported systems (Avature, Phenom,
  SuccessFactors, their own sites) still need the link pasted by hand.
- **Interests aren't copied to Add Jobs.** A company filter only has roles,
  locations and experience.
- **Shared directory.** `/nearby` (green pins) covers the caller's own
  companies. `osm_places` is the start of the plan's shared directory, but
  it has no career pages, so it can't be checked for jobs yet.
- **Go Online** is still UI only. Next: `presence` table, matching, Web Push
  (also free, with VAPID keys).
- `closed_at` is never set yet. The sync only sees jobs that pass the
  filter, so it can't tell "closed" from "no longer matching".

## How to try it

1. Apply migrations 007 and 008 (see the README).
2. Import offices (README, "Importing nearby companies"): download
   `karnataka.osm.pbf`, then
   `$env:CGO_ENABLED="0"; go run ./cmd/importplaces -file karnataka.osm.pbf`
   in `cron-fetch`.
3. Restart the API (`go run ./cmd/api`) so it has the new routes.
4. `npm run dev` in `crontfetch-web`, open **Jobs around me**, allow
   location. Grey pins and the **Companies around you** list appear.
5. Click **Track** on one. Add Jobs opens; the career page fills in after a
   second or so (check the note under it), and roles, locations and
   experience come from your filter. Save. It comes back as a green pin.
6. To fake moving around: Chrome DevTools → ⋮ → More tools → Sensors →
   Location, then **Refresh location**.
