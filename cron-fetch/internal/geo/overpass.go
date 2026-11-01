package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OfficeTypes are the OpenStreetMap office=* values that are likely to be
// employers worth showing on Jobs around me.
var OfficeTypes = []string{"company", "it", "software", "telecommunication", "engineering", "consulting", "research", "financial"}

// IsOffice reports whether an OSM element's tags describe a named office of
// one of the OfficeTypes.
func IsOffice(tags map[string]string) bool {
	if strings.TrimSpace(tags["name"]) == "" {
		return false
	}
	for _, t := range OfficeTypes {
		if tags["office"] == t {
			return true
		}
	}
	return false
}

// PlaceFromTags builds an OSMPlace from an element's ID (e.g. "osm:way/2"),
// tags and position.
func PlaceFromTags(id string, tags map[string]string, lat, lng float64) OSMPlace {
	return OSMPlace{
		ID:      id,
		Name:    strings.TrimSpace(tags["name"]),
		Kind:    tags["office"],
		Website: firstNonEmpty(tags["website"], tags["contact:website"], tags["url"]),
		Address: osmAddress(tags),
		Lat:     lat,
		Lng:     lng,
	}
}

// Overpass queries the free Overpass API. It returns the most complete list
// when it's up, but the public servers are often overloaded, so several
// mirrors are tried in turn (starting with whichever answered last), all
// within Budget so a fallback source still has time to run.
type Overpass struct {
	Mirrors          []string
	UserAgent        string
	Client           *http.Client
	PerMirrorTimeout time.Duration
	Budget           time.Duration

	preferred int // index into Mirrors of the last one that worked; Places serialises calls
}

// NewOverpass returns a client for the public Overpass servers.
func NewOverpass() *Overpass {
	return &Overpass{
		Mirrors: []string{
			"https://overpass-api.de/api/interpreter",
			"https://maps.mail.ru/osm/tools/overpass/api/interpreter",
			"https://overpass.private.coffee/api/interpreter",
			"https://overpass.kumi.systems/api/interpreter",
		},
		UserAgent:        userAgent,
		Client:           &http.Client{},
		PerMirrorTimeout: 12 * time.Second,
		Budget:           25 * time.Second,
	}
}

func (o *Overpass) Name() string { return "overpass" }

// Fetch tries each mirror in turn and remembers whichever answers.
func (o *Overpass) Fetch(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, error) {
	ctx, cancel := context.WithTimeout(ctx, o.Budget)
	defer cancel()

	var errs []string
	for i := range o.Mirrors {
		idx := (o.preferred + i) % len(o.Mirrors)
		mctx, mcancel := context.WithTimeout(ctx, o.PerMirrorTimeout)
		places, err := o.query(mctx, o.Mirrors[idx], lat, lng, radiusKm)
		mcancel()
		if err == nil {
			o.preferred = idx
			return places, nil
		}
		slog.Warn("overpass: mirror failed", "mirror", o.Mirrors[idx], "error", err)
		errs = append(errs, err.Error())
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("all mirrors failed: %s", strings.Join(errs, "; "))
}

func (o *Overpass) query(ctx context.Context, endpoint string, lat, lng, radiusKm float64) ([]OSMPlace, error) {
	q := fmt.Sprintf(`[out:json][timeout:20];
nwr["office"~"^(%s)$"]["name"](around:%.0f,%f,%f);
out center tags 2000;`, strings.Join(OfficeTypes, "|"), radiusKm*1000, lat, lng)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(url.Values{"data": {q}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", o.UserAgent)

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
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
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	places := make([]OSMPlace, 0, len(body.Elements))
	for _, e := range body.Elements {
		pLat, pLng := e.Lat, e.Lon
		if e.Center != nil { // ways and relations come back with a centre point
			pLat, pLng = e.Center.Lat, e.Center.Lon
		}
		places = append(places, PlaceFromTags(fmt.Sprintf("osm:%s/%d", e.Type, e.ID), e.Tags, pLat, pLng))
	}
	return places, nil
}

func osmAddress(tags map[string]string) string {
	if full := tags["addr:full"]; full != "" {
		return full
	}
	return joinNonEmpty(tags["addr:housenumber"], tags["addr:street"], tags["addr:suburb"], tags["addr:city"])
}

func joinNonEmpty(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
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
