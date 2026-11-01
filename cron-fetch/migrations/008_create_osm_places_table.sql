-- Offices imported from an OpenStreetMap extract (cmd/importplaces), so
-- "companies around me" doesn't depend on public map servers at request time.
CREATE TABLE IF NOT EXISTS osm_places (
    id TEXT PRIMARY KEY,                 -- e.g. osm:node/123, osm:way/456
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT '',       -- the office=* value
    website TEXT NOT NULL DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    lat DOUBLE PRECISION NOT NULL,
    lng DOUBLE PRECISION NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS osm_places_lat_lng_idx ON osm_places (lat, lng);
