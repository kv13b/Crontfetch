package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// officeTypes are the OpenStreetMap office=* values that are likely to be
// employers worth showing on Jobs around me.
const officeTypes = "company|it|software|telecommunication|engineering|consulting|research|financial|insurance|estate_agent|advertising_agency|logistics"

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
}

// Overpass finds offices near a point using the free Overpass API.
// Results are cached per ~1 km grid cell for a day, and only one query runs
// at a time, to stay well within the public server's fair-use limits.
type Overpass struct {
	BaseURL   string
	UserAgent string
	Client    *http.Client
	TTL       time.Duration

	queryMu sync.Mutex // one upstream query at a time

	mu    sync.Mutex
	cache map[string]overpassEntry
}

type overpassEntry struct {
	places  []OSMPlace
	fetched time.Time
}

// NewOverpass returns a client for the public overpass-api.de server.
func NewOverpass() *Overpass {
	return &Overpass{
		BaseURL:   "https://overpass-api.de/api/interpreter",
		UserAgent: "CronFetch/0.1 (https://github.com/kv13b/Crontfetch)",
		Client:    &http.Client{Timeout: 40 * time.Second},
		TTL:       24 * time.Hour,
		cache:     make(map[string]overpassEntry),
	}
}

// Offices returns named offices within radiusKm of lat/lng, closest first.
func (o *Overpass) Offices(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, error) {
	// Snap to a ~1 km grid so small moves reuse the cached result. Querying
	// a slightly bigger radius around the cell centre covers the snap error.
	cLat, cLng := math.Round(lat*100)/100, math.Round(lng*100)/100
	key := fmt.Sprintf("%.2f,%.2f,%.0f", cLat, cLng, radiusKm)

	places, ok := o.cached(key)
	if !ok {
		o.queryMu.Lock()
		// Another request may have filled the cache while this one waited.
		if places, ok = o.cached(key); !ok {
			var err error
			places, err = o.query(ctx, cLat, cLng, radiusKm+1)
			if err != nil {
				o.queryMu.Unlock()
				return nil, err
			}
			o.mu.Lock()
			o.cache[key] = overpassEntry{places: places, fetched: time.Now()}
			o.mu.Unlock()
		}
		o.queryMu.Unlock()
	}

	out := make([]OSMPlace, 0, len(places))
	for _, p := range places {
		p.DistanceKm = DistanceKm(lat, lng, p.Lat, p.Lng)
		if p.DistanceKm <= radiusKm {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DistanceKm < out[j].DistanceKm })
	return out, nil
}

func (o *Overpass) cached(key string) ([]OSMPlace, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	e, ok := o.cache[key]
	if !ok || time.Since(e.fetched) > o.TTL {
		return nil, false
	}
	return e.places, true
}

func (o *Overpass) query(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, error) {
	q := fmt.Sprintf(`[out:json][timeout:30];
nwr["office"~"^(%s)$"]["name"](around:%.0f,%f,%f);
out center tags 500;`, officeTypes, radiusKm*1000, lat, lng)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL, strings.NewReader(url.Values{"data": {q}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", o.UserAgent)

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("overpass: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass: status %d", resp.StatusCode)
	}

	var body struct {
		Elements []struct {
			Type   string                      `json:"type"`
			ID     int64                       `json:"id"`
			Lat    float64                     `json:"lat"`
			Lon    float64                     `json:"lon"`
			Center *struct{ Lat, Lon float64 } `json:"center"`
			Tags   map[string]string           `json:"tags"`
		} `json:"elements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("overpass: decoding response: %w", err)
	}

	places := make([]OSMPlace, 0, len(body.Elements))
	for _, e := range body.Elements {
		pLat, pLng := e.Lat, e.Lon
		if e.Center != nil { // ways and relations come back with a centre point
			pLat, pLng = e.Center.Lat, e.Center.Lon
		}
		if pLat == 0 && pLng == 0 {
			continue
		}
		places = append(places, OSMPlace{
			ID:      fmt.Sprintf("osm:%s/%d", e.Type, e.ID),
			Name:    e.Tags["name"],
			Kind:    e.Tags["office"],
			Website: firstNonEmpty(e.Tags["website"], e.Tags["contact:website"], e.Tags["url"]),
			Address: osmAddress(e.Tags),
			Lat:     pLat,
			Lng:     pLng,
		})
	}
	return places, nil
}

func osmAddress(tags map[string]string) string {
	if full := tags["addr:full"]; full != "" {
		return full
	}
	parts := []string{}
	for _, k := range []string{"addr:housenumber", "addr:street", "addr:suburb", "addr:city"} {
		if v := tags[k]; v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
