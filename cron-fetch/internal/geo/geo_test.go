package geo

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDistanceKm(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lng1, lat2, lng2 float64
		want                   float64
	}{
		{"same point", 12.9716, 77.5946, 12.9716, 77.5946, 0},
		// MG Road to Koramangala, Bangalore: about 4.9 km.
		{"across Bangalore", 12.9756, 77.6066, 12.9352, 77.6245, 4.9},
		{"Bangalore to Delhi", 12.9716, 77.5946, 28.6139, 77.2090, 1740},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DistanceKm(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			if math.Abs(got-tt.want) > tt.want*0.02+0.01 {
				t.Errorf("DistanceKm() = %.2f, want about %.2f", got, tt.want)
			}
		})
	}
}

func TestValidCoords(t *testing.T) {
	if !ValidCoords(12.97, 77.59) || ValidCoords(91, 0) || ValidCoords(0, -181) {
		t.Error("ValidCoords range check is wrong")
	}
}

func TestNominatim_Geocode(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		if r.URL.Query().Get("q") == "nowhere" {
			w.Write([]byte(`[]`))
			return
		}
		w.Write([]byte(`[{"lat":"12.9716","lon":"77.5946","place_id":42}]`))
	}))
	defer srv.Close()

	n := NewNominatim()
	n.BaseURL = srv.URL

	p, err := n.Geocode(context.Background(), "MG Road, Bangalore")
	if err != nil {
		t.Fatal(err)
	}
	if p.Lat != 12.9716 || p.Lng != 77.5946 || p.PlaceID != "osm:42" {
		t.Errorf("Geocode() = %+v", p)
	}
	if gotUA == "" {
		t.Error("User-Agent must be set (Nominatim policy)")
	}

	if _, err := n.Geocode(context.Background(), "nowhere"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Geocode(nowhere) error = %v, want ErrNotFound", err)
	}
}
