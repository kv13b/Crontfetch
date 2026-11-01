package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// photonKeywords are words common in company names. Photon caps every
// lookup at 50 results, and its "nearest" lookup only reaches a kilometre or
// two in a dense area, so these searches across the whole area fill in the rest.
var photonKeywords = []string{
	"technologies", "technology", "software", "solutions", "systems", "labs",
	"services", "consulting", "india", "infotech", "private", "limited",
	"tech", "digital", "global", "networks", "engineering", "analytics",
}

// Photon looks up offices with komoot's free Photon search over
// OpenStreetMap data. It's fast and usually up, but less complete than
// Overpass, so it's the fallback.
type Photon struct {
	BaseURL     string
	UserAgent   string
	Client      *http.Client
	Concurrency int
}

// NewPhoton returns a client for the public photon.komoot.io server.
func NewPhoton() *Photon {
	return &Photon{
		BaseURL:     "https://photon.komoot.io",
		UserAgent:   userAgent,
		Client:      &http.Client{Timeout: 15 * time.Second},
		Concurrency: 2,
	}
}

func (p *Photon) Name() string { return "photon" }

// Fetch runs the nearest-offices lookup plus one search per keyword, limited
// to the area's bounding box, and merges the results. It fails if the
// nearest lookup fails; a failed keyword search just leaves its results out.
func (p *Photon) Fetch(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, error) {
	dLat := radiusKm / 111.0
	dLng := radiusKm / (111.0 * math.Cos(lat*math.Pi/180))
	bbox := fmt.Sprintf("%f,%f,%f,%f", lng-dLng, lat-dLat, lng+dLng, lat+dLat)

	tags := url.Values{}
	for _, t := range OfficeTypes {
		tags.Add("osm_tag", "office:"+t)
	}

	// Nearest offices first, on its own: if Photon is down or throttling us,
	// give up now instead of waiting out a timeout for every keyword.
	reverse := url.Values{"lat": {ftoa(lat)}, "lon": {ftoa(lng)}, "radius": {ftoa(radiusKm)}, "limit": {"50"}}
	places, err := p.get(ctx, p.BaseURL+"/reverse?"+merge(reverse, tags).Encode())
	if err != nil {
		return nil, err
	}

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, max(1, p.Concurrency))
	)
	for _, kw := range photonKeywords {
		search := url.Values{"q": {kw}, "bbox": {bbox}, "limit": {"50"}}
		u := p.BaseURL + "/api/?" + merge(search, tags).Encode()
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			got, err := p.get(ctx, u)
			if err != nil {
				slog.Warn("photon: search failed", "keyword", kw, "error", err)
				return
			}
			mu.Lock()
			places = append(places, got...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return places, nil
}

func (p *Photon) get(ctx context.Context, u string) ([]OSMPlace, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.UserAgent)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var body struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"` // [lng, lat]
			} `json:"geometry"`
			Properties struct {
				Name        string `json:"name"`
				OSMType     string `json:"osm_type"`
				OSMID       int64  `json:"osm_id"`
				OSMValue    string `json:"osm_value"`
				HouseNumber string `json:"housenumber"`
				Street      string `json:"street"`
				District    string `json:"district"`
				Locality    string `json:"locality"`
				City        string `json:"city"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	places := make([]OSMPlace, 0, len(body.Features))
	for _, f := range body.Features {
		if len(f.Geometry.Coordinates) != 2 {
			continue
		}
		pr := f.Properties
		places = append(places, OSMPlace{
			ID:      fmt.Sprintf("osm:%s/%d", photonOSMType(pr.OSMType), pr.OSMID),
			Name:    pr.Name,
			Kind:    pr.OSMValue,
			Address: joinNonEmpty(pr.HouseNumber, pr.Street, firstNonEmpty(pr.District, pr.Locality), pr.City),
			Lat:     f.Geometry.Coordinates[1],
			Lng:     f.Geometry.Coordinates[0],
		})
	}
	return places, nil
}

// photonOSMType maps Photon's N/W/R to the names Overpass uses, so the same
// office gets the same ID from either source.
func photonOSMType(t string) string {
	switch t {
	case "N":
		return "node"
	case "W":
		return "way"
	case "R":
		return "relation"
	}
	return t
}

func merge(a, b url.Values) url.Values {
	out := url.Values{}
	for k, v := range a {
		out[k] = append(out[k], v...)
	}
	for k, v := range b {
		out[k] = append(out[k], v...)
	}
	return out
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
