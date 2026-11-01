package geo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// OSMPlace is a named office from OpenStreetMap.
type OSMPlace struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Website    string  `json:"website"`
	Address    string  `json:"address"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	DistanceKm float64 `json:"distance_km"`
	Source     string  `json:"source"`
}

// Source returns the offices it knows of within radiusKm of a point. Results
// may include some just outside the radius; Places filters them.
type Source interface {
	Name() string
	Fetch(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, error)
}

// ErrNoData means a source has nothing for this area (e.g. the local import
// doesn't cover it). The next source is tried, but this isn't a failure, so
// the source isn't put in its cooldown.
var ErrNoData = errors.New("no data for this area")

// uncached is implemented by sources that are cheap to query, like the local
// database, so their results are always fresh (e.g. right after an import).
type uncached interface {
	Uncached() bool
}

// Places finds offices near a point by asking each source in order until
// one succeeds. Results are cached per ~5 km grid cell for a day, and only
// one upstream lookup runs at a time, to stay within the free services'
// fair-use limits.
type Places struct {
	Sources []Source
	TTL     time.Duration
	// Cooldown is how long a source that just failed is skipped, so a down
	// service doesn't add its full timeout to every lookup.
	Cooldown time.Duration

	fetchMu   sync.Mutex           // one upstream lookup at a time
	downUntil map[string]time.Time // guarded by fetchMu

	mu    sync.Mutex
	cache map[string]placesEntry
}

type placesEntry struct {
	places  []OSMPlace
	fetched time.Time
}

// cellDeg is the cache grid size (~5.5 km). Lookups cover the radius plus
// the cell's half-diagonal, so any point in a cell is fully served.
const (
	cellDeg   = 0.05
	cellPadKm = 4
)

// NewPlaces returns a finder that tries the sources in the order given.
func NewPlaces(sources ...Source) *Places {
	return &Places{
		Sources:   sources,
		TTL:       24 * time.Hour,
		Cooldown:  10 * time.Minute,
		downUntil: make(map[string]time.Time),
		cache:     make(map[string]placesEntry),
	}
}

// Offices returns named offices within radiusKm of lat/lng, closest first.
func (p *Places) Offices(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, error) {
	// Snap to the grid so moving around reuses the cached result.
	cLat, cLng := math.Round(lat/cellDeg)*cellDeg, math.Round(lng/cellDeg)*cellDeg
	key := fmt.Sprintf("%.2f,%.2f,%.0f", cLat, cLng, radiusKm)

	places, ok := p.cached(key)
	if !ok {
		p.fetchMu.Lock()
		// Another request may have filled the cache while this one waited.
		if places, ok = p.cached(key); !ok {
			var (
				err       error
				cacheable bool
			)
			places, cacheable, err = p.fetch(ctx, cLat, cLng, radiusKm+cellPadKm)
			if err != nil {
				p.fetchMu.Unlock()
				return nil, err
			}
			if cacheable {
				p.mu.Lock()
				p.cache[key] = placesEntry{places: places, fetched: time.Now()}
				p.mu.Unlock()
			}
		}
		p.fetchMu.Unlock()
	}

	out := make([]OSMPlace, 0, len(places))
	for _, pl := range places {
		pl.DistanceKm = DistanceKm(lat, lng, pl.Lat, pl.Lng)
		if pl.DistanceKm <= radiusKm {
			out = append(out, pl)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DistanceKm < out[j].DistanceKm })
	return out, nil
}

func (p *Places) cached(key string) ([]OSMPlace, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.cache[key]
	if !ok || time.Since(e.fetched) > p.TTL {
		return nil, false
	}
	return e.places, true
}

// fetch asks each source in turn and returns the first success, deduped,
// and whether it may be cached. A source in its cooldown is skipped, unless
// it's the last one left. Call with fetchMu held.
func (p *Places) fetch(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, bool, error) {
	var errs []error
	for i, s := range p.Sources {
		if time.Now().Before(p.downUntil[s.Name()]) && i < len(p.Sources)-1 {
			continue
		}
		places, err := s.Fetch(ctx, lat, lng, radiusKm)
		if err == nil {
			slog.Info("places: fetched", "source", s.Name(), "count", len(places), "lat", lat, "lng", lng)
			for i := range places {
				places[i].Source = s.Name()
			}
			u, ok := s.(uncached)
			return dedupe(places), !ok || !u.Uncached(), nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
		if errors.Is(err, ErrNoData) {
			slog.Info("places: source has no data here, trying next", "source", s.Name(), "lat", lat, "lng", lng)
			continue
		}
		slog.Warn("places: source failed", "source", s.Name(), "error", err, "skipping_for", p.Cooldown)
		p.downUntil[s.Name()] = time.Now().Add(p.Cooldown)
		if ctx.Err() != nil {
			break
		}
	}
	return nil, false, errors.Join(errs...)
}

// dedupe drops repeats of the same office. OSM often maps one office twice
// (a point and a building outline), and Photon's searches overlap.
func dedupe(places []OSMPlace) []OSMPlace {
	seen := make(map[string]bool, len(places))
	out := places[:0]
	for _, pl := range places {
		key := fmt.Sprintf("%s|%.3f|%.3f", strings.ToLower(strings.TrimSpace(pl.Name)), pl.Lat, pl.Lng)
		if pl.Name == "" || (pl.Lat == 0 && pl.Lng == 0) || seen[key] || seen[pl.ID] {
			continue
		}
		seen[key], seen[pl.ID] = true, true
		out = append(out, pl)
	}
	return out
}
