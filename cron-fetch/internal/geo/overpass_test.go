package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOverpass_Offices(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"elements":[
			{"type":"node","id":1,"lat":12.9750,"lon":77.6060,"tags":{"name":"Near Co","office":"it","website":"https://near.example"}},
			{"type":"way","id":2,"center":{"lat":12.9352,"lon":77.6245},"tags":{"name":"Way Co","office":"company","addr:street":"80 Feet Rd","addr:city":"Bengaluru"}},
			{"type":"node","id":3,"lat":13.2,"lon":77.9,"tags":{"name":"Too Far","office":"company"}}
		]}`))
	}))
	defer srv.Close()

	o := NewOverpass()
	o.BaseURL = srv.URL

	got, err := o.Offices(context.Background(), 12.9716, 77.5946, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "Near Co" || got[1].Name != "Way Co" {
		t.Fatalf("Offices() = %+v, want Near Co then Way Co", got)
	}
	if got[0].Website != "https://near.example" || got[1].Address != "80 Feet Rd, Bengaluru" || got[1].ID != "osm:way/2" {
		t.Errorf("fields not mapped: %+v", got)
	}

	// A small move stays in the same grid cell and is served from cache.
	if _, err := o.Offices(context.Background(), 12.9718, 77.5949, 10); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("upstream calls = %d, want 1 (second request should hit the cache)", calls)
	}
}
