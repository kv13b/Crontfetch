package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPhoton_Fetch(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if len(r.URL.Query()["osm_tag"]) == 0 {
			t.Error("osm_tag filter missing")
		}
		if strings.HasPrefix(r.URL.Path, "/reverse") {
			w.Write([]byte(`{"features":[{"geometry":{"coordinates":[77.6060,12.9750]},
				"properties":{"name":"Cisco","osm_type":"W","osm_id":7,"osm_value":"company","street":"Outer Ring Rd","district":"Panathur","city":"Bengaluru"}}]}`))
			return
		}
		if r.URL.Query().Get("bbox") == "" {
			t.Error("search must be limited to the area's bbox")
		}
		w.Write([]byte(`{"features":[{"geometry":{"coordinates":[77.6245,12.9352]},
			"properties":{"name":"Thoughtworks","osm_type":"N","osm_id":9,"osm_value":"company"}}]}`))
	}))
	defer srv.Close()

	p := NewPhoton()
	p.BaseURL = srv.URL

	got, err := p.Fetch(context.Background(), 12.9716, 77.5946, 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := int32(1 + len(photonKeywords)); calls.Load() != want {
		t.Errorf("made %d requests, want %d (nearest + one per keyword)", calls.Load(), want)
	}

	var cisco *OSMPlace
	for i := range got {
		if got[i].Name == "Cisco" {
			cisco = &got[i]
		}
	}
	if cisco == nil {
		t.Fatal("Cisco from the nearest lookup is missing")
	}
	if cisco.ID != "osm:way/7" || cisco.Lat != 12.9750 || cisco.Lng != 77.6060 || cisco.Address != "Outer Ring Rd, Panathur, Bengaluru" {
		t.Errorf("Cisco not mapped: %+v", *cisco)
	}
}

func TestPhoton_FailsWhenNearestLookupFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := NewPhoton()
	p.BaseURL = srv.URL
	if _, err := p.Fetch(context.Background(), 12.97, 77.59, 10); err == nil {
		t.Error("Fetch() should fail when the nearest lookup fails")
	}
}
