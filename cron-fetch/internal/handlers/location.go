package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/geo"
	"github.com/kv13b/Crontfetch/internal/middleware"
	"github.com/kv13b/Crontfetch/internal/models"
)

const (
	defaultNearbyRadiusKm = 10.0
	maxNearbyRadiusKm     = 50.0
	geocodeTimeout        = 15 * time.Second
	placesTimeout         = 45 * time.Second
)

// Geocoder turns an address into coordinates. geo.Nominatim satisfies it.
type Geocoder interface {
	Geocode(ctx context.Context, address string) (geo.Place, error)
}

type setLocationRequest struct {
	OfficeAddress string   `json:"office_address"`
	Lat           *float64 `json:"lat"`
	Lng           *float64 `json:"lng"`
}

// SetCompanyLocation handles PUT /companies/{id}/location. Coordinates sent
// directly (e.g. a pin dropped on the map) are used as-is; otherwise the
// office address is geocoded. An empty body clears the location.
func SetCompanyLocation(pool *pgxpool.Pool, geocoder Geocoder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		var req setLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req.OfficeAddress = strings.TrimSpace(req.OfficeAddress)

		var place *geo.Place
		switch {
		case (req.Lat == nil) != (req.Lng == nil):
			writeError(w, http.StatusBadRequest, "lat and lng must be sent together")
			return
		case req.Lat != nil:
			if !geo.ValidCoords(*req.Lat, *req.Lng) {
				writeError(w, http.StatusBadRequest, "lat must be -90..90 and lng -180..180")
				return
			}
			place = &geo.Place{Lat: *req.Lat, Lng: *req.Lng}
		case req.OfficeAddress != "":
			ctx, cancel := context.WithTimeout(r.Context(), geocodeTimeout)
			p, err := geocoder.Geocode(ctx, req.OfficeAddress)
			cancel()
			if errors.Is(err, geo.ErrNotFound) {
				writeError(w, http.StatusBadRequest, "couldn't find that address; try a fuller address or drop a pin on the map")
				return
			}
			if err != nil {
				slog.Error("company location: geocoding", "error", err, "address", req.OfficeAddress)
				writeError(w, http.StatusBadGateway, "address lookup failed, try again or drop a pin on the map")
				return
			}
			place = &p
		}

		company, err := updateCompanyLocation(r.Context(), pool, claims.UserID, r.PathValue("id"), req.OfficeAddress, place)
		if err != nil {
			if isCompanyNotFound(err) {
				writeError(w, http.StatusNotFound, "company not found")
				return
			}
			slog.Error("company location: updating", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not save location")
			return
		}

		slog.Info("company: location set", "company_id", company.ID, "user_id", claims.UserID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(company)
	}
}

// updateCompanyLocation saves the address and coordinates, clearing the
// coordinates when place is nil.
func updateCompanyLocation(ctx context.Context, pool *pgxpool.Pool, userID, id, address string, place *geo.Place) (models.Company, error) {
	const query = `
		UPDATE companies
		SET office_address = $3, lat = $4, lng = $5, place_id = $6,
		    geocoded_at = CASE WHEN $4::double precision IS NULL THEN NULL ELSE now() END,
		    updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING ` + companyColumns

	var lat, lng *float64
	placeID := ""
	if place != nil {
		lat, lng, placeID = &place.Lat, &place.Lng, place.PlaceID
	}
	return scanCompany(pool.QueryRow(ctx, query, id, userID, address, lat, lng, placeID))
}

type nearbyCompany struct {
	models.Company
	DistanceKm float64 `json:"distance_km"`
}

// Nearby handles GET /nearby?lat=&lng=&radius_km=: the caller's companies
// with an office within the radius (default 10 km), closest first.
func Nearby(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		lat, lng, radius, msg := parseNearbyQuery(r)
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}

		companies, err := listCompaniesByUser(r.Context(), pool, claims.UserID)
		if err != nil {
			slog.Error("nearby: listing companies", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not list companies")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(filterNearby(companies, lat, lng, radius))
	}
}

// parseNearbyQuery reads lat, lng and the optional radius_km, returning a
// message for a 400 response if they're missing or out of range.
func parseNearbyQuery(r *http.Request) (lat, lng, radius float64, msg string) {
	q := r.URL.Query()
	lat, errLat := strconv.ParseFloat(q.Get("lat"), 64)
	lng, errLng := strconv.ParseFloat(q.Get("lng"), 64)
	if errLat != nil || errLng != nil || !geo.ValidCoords(lat, lng) {
		return 0, 0, 0, "lat and lng query parameters are required"
	}
	radius = defaultNearbyRadiusKm
	if raw := q.Get("radius_km"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 || v > maxNearbyRadiusKm {
			return 0, 0, 0, "radius_km must be between 0 and 50"
		}
		radius = v
	}
	return lat, lng, radius, ""
}

// PlaceFinder lists offices near a point. geo.Overpass satisfies it.
type PlaceFinder interface {
	Offices(ctx context.Context, lat, lng, radiusKm float64) ([]geo.OSMPlace, error)
}

// NearbyPlaces handles GET /nearby/places?lat=&lng=&radius_km=: named offices
// from OpenStreetMap around the point, whether or not the user tracks them.
// Kept separate from /nearby because the upstream lookup can take seconds.
func NearbyPlaces(finder PlaceFinder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lat, lng, radius, msg := parseNearbyQuery(r)
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), placesTimeout)
		defer cancel()
		places, err := finder.Offices(ctx, lat, lng, radius)
		if err != nil {
			slog.Error("nearby places: overpass", "error", err)
			writeError(w, http.StatusBadGateway, "map data service is busy, try again in a minute")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(places)
	}
}

// filterNearby keeps companies with coordinates within radiusKm, closest first.
func filterNearby(companies []models.Company, lat, lng, radiusKm float64) []nearbyCompany {
	nearby := make([]nearbyCompany, 0)
	for _, c := range companies {
		if c.Lat == nil || c.Lng == nil {
			continue
		}
		if d := geo.DistanceKm(lat, lng, *c.Lat, *c.Lng); d <= radiusKm {
			nearby = append(nearby, nearbyCompany{Company: c, DistanceKm: d})
		}
	}
	sort.Slice(nearby, func(i, j int) bool { return nearby[i].DistanceKm < nearby[j].DistanceKm })
	return nearby
}
