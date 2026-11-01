# Maps and Jobs around me — build log

Status as of 2026-10-31. Covers the first slice of the "online" features in
[execution-plan.md](execution-plan.md): company locations, the nearby
search, and the map on **Jobs around me**. Nothing here is committed yet.

## Goal: no paid services

The plan originally used Google Maps and Google Geocoding. Everything below
uses free services instead:

| Need | Free option used | Limits |
|---|---|---|
| Map display | Leaflet + OpenStreetMap tiles | Attribution required (shown on the map). |
| Address → coordinates | Nominatim (OpenStreetMap) | Max 1 request/second, identifying User-Agent. The server enforces both. |
| Real companies around a point | Overpass API (OpenStreetMap) | Fair use. Results cached for a day per ~1 km grid cell, one query at a time. |
| User's position | Browser Geolocation API | Works on `localhost` and HTTPS only. |

## What was built

### Database

`cron-fetch/migrations/007_add_geo_and_job_status.sql` (applied to the local
Docker database):

- `companies`: `office_address`, `lat`, `lng`, `place_id`, `geocoded_at`.
- `jobs`: `last_seen_at`, `closed_at`.

### Backend (`cron-fetch`)

| Piece | What it does |
|---|---|
| `internal/geo/geo.go` | `DistanceKm` (haversine), `ValidCoords`, and a Nominatim geocoder. |
| `internal/geo/overpass.go` | Finds named offices (`office=company`, `it`, `software`, …) near a point via Overpass, with a 24-hour cache. |
| `internal/handlers/location.go` | The three new routes below. |
| `internal/jobsync/db.go` | A sync now refreshes `last_seen_at` (and clears `closed_at`) on jobs it sees again. "New job" detection is unchanged. |
| `internal/models/company.go` | Company JSON now includes `office_address`, `lat`, `lng`. |

New routes (all need the JWT):

| Method | Path | Purpose |
|---|---|---|
| PUT | `/companies/{id}/location` | Set an office location: send `lat`/`lng` (a map pin) or just `office_address` (looked up with Nominatim). `{}` clears it. |
| GET | `/nearby?lat=&lng=&radius_km=` | The caller's tracked companies within the radius (default 10 km, max 50), closest first. |
| GET | `/nearby/places?lat=&lng=&radius_km=` | Real offices from OpenStreetMap around the point, tracked or not. |

Full request/response details are in
[cron-fetch/docs/api-reference.md](../cron-fetch/docs/api-reference.md).

### Frontend (`crontfetch-web`)

- Added `leaflet`, `react-leaflet`, `@types/leaflet`.
- `src/services/nearbyService.ts`: `fetchNearby`, `setCompanyLocation`.
- `src/components/NearbyMap.tsx`: map with your position, the 10 km circle,
  and a green pin per nearby tracked company (popup with distance and career
  page link).
- `src/pages/NearbyPage.tsx`: replaces the placeholder. Shows the map, the
  nearby list, and an **Office locations** section where each tracked
  company gets a location by typing an address (**Find address**) or by
  **Pick on map**. If location is blocked, it falls back to central Bangalore.
- `src/styles/pages.css`: styles for the above.

## Tested

- Unit tests: distance, coordinate range, Nominatim parsing, Overpass parsing
  and caching, nearby filtering and ordering. All pass.
- Against the real API and local database (throwaway account, deleted
  afterwards): pins at 1.4 km and 5.2 km returned closest first, a Delhi
  pin excluded; a real Nominatim lookup of "Koramangala, Bengaluru" returned
  12.9357, 77.6241; bad input returns clear `400`s.
- Frontend production build passes.

## Not done yet

- **Grey OpenStreetMap pins on the map.** `GET /nearby/places` exists, but
  the frontend doesn't call it yet. Planned: grey pins next to the green
  tracked ones, each with a **Track this company** button that opens Add
  Jobs pre-filled with the name, website and location.
- **Live Overpass check.** A test query from this machine to
  `overpass-api.de` timed out (40 s). The public server is often slow; if it
  keeps failing, switch `BaseURL` to a mirror such as
  `overpass.private.coffee`, or shrink the query.
- **Shared directory.** `/nearby` covers the caller's own companies only.
  The plan's shared company directory comes later.
- **Go Online** is still UI only. Next steps from the plan: `presence`
  table, then matching, then Web Push (also free, with VAPID keys).
- `closed_at` is never set yet. The sync only sees jobs that pass the
  filter, so it can't tell "closed" from "no longer matching".

## How to try it

1. Apply migration 007 if your database doesn't have it (see the README).
2. Restart the API (`go run ./cmd/api` in `cron-fetch`). An API started
   before these changes returns `404` for `/nearby`, which the page shows as
   "Something went wrong".
3. `npm run dev` in `crontfetch-web`, open **Jobs around me**, allow location.
4. Add a company under **Add Jobs**, then give it a location in **Office
   locations**. A green pin appears if it's within 10 km.
5. To fake moving around: Chrome DevTools → ⋮ → More tools → Sensors →
   Location, then **Refresh location**.
