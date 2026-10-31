// Package geo does the location maths for nearby companies and turns office
// addresses into coordinates using OpenStreetMap's free Nominatim service.
package geo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const earthRadiusKm = 6371.0

// DistanceKm is the great-circle (haversine) distance between two points.
func DistanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := rad(lat2 - lat1)
	dLng := rad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}

// ValidCoords reports whether lat/lng are inside the real-world range.
func ValidCoords(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// ErrNotFound means the geocoder had no result for the address.
var ErrNotFound = errors.New("address not found")

// Place is a geocoding result.
type Place struct {
	Lat     float64
	Lng     float64
	PlaceID string
}

// Nominatim geocodes addresses with nominatim.openstreetmap.org. Its usage
// policy allows at most one request per second and requires an identifying
// User-Agent, so calls are serialised and spaced out.
type Nominatim struct {
	BaseURL   string
	UserAgent string
	Client    *http.Client

	mu   sync.Mutex
	last time.Time
}

// NewNominatim returns a geocoder for the public Nominatim service.
func NewNominatim() *Nominatim {
	return &Nominatim{
		BaseURL:   "https://nominatim.openstreetmap.org",
		UserAgent: "CronFetch/0.1 (https://github.com/kv13b/Crontfetch)",
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Geocode returns the best match for address.
func (n *Nominatim) Geocode(ctx context.Context, address string) (Place, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if wait := time.Second - time.Since(n.last); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return Place{}, ctx.Err()
		}
	}
	defer func() { n.last = time.Now() }()

	q := url.Values{"q": {address}, "format": {"jsonv2"}, "limit": {"1"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.BaseURL+"/search?"+q.Encode(), nil)
	if err != nil {
		return Place{}, err
	}
	req.Header.Set("User-Agent", n.UserAgent)

	resp, err := n.Client.Do(req)
	if err != nil {
		return Place{}, fmt.Errorf("geocoding: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Place{}, fmt.Errorf("geocoding: status %d", resp.StatusCode)
	}

	// Nominatim returns lat/lon as strings.
	var results []struct {
		Lat     string `json:"lat"`
		Lon     string `json:"lon"`
		PlaceID int64  `json:"place_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return Place{}, fmt.Errorf("geocoding: decoding response: %w", err)
	}
	if len(results) == 0 {
		return Place{}, ErrNotFound
	}

	lat, err1 := strconv.ParseFloat(results[0].Lat, 64)
	lng, err2 := strconv.ParseFloat(results[0].Lon, 64)
	if err1 != nil || err2 != nil {
		return Place{}, fmt.Errorf("geocoding: bad coordinates %q,%q", results[0].Lat, results[0].Lon)
	}
	return Place{Lat: lat, Lng: lng, PlaceID: "osm:" + strconv.FormatInt(results[0].PlaceID, 10)}, nil
}
