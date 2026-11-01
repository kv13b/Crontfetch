package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOverpass_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent must be set (fair-use policy)")
		}
		w.Write([]byte(`{"elements":[
			{"type":"node","id":1,"lat":12.9750,"lon":77.6060,"tags":{"name":"Near Co","office":"it","website":"https://near.example"}},
			{"type":"way","id":2,"center":{"lat":12.9352,"lon":77.6245},"tags":{"name":"Way Co","office":"company","addr:street":"80 Feet Rd","addr:city":"Bengaluru"}}
		]}`))
	}))
	defer srv.Close()

	o := NewOverpass()
	o.Mirrors = []string{srv.URL}

	got, err := o.Fetch(context.Background(), 12.9716, 77.5946, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("Fetch() returned %d places, want 2", len(got))
	}
	if got[0].Website != "https://near.example" || got[0].ID != "osm:node/1" {
		t.Errorf("node not mapped: %+v", got[0])
	}
	if got[1].Lat != 12.9352 || got[1].Address != "80 Feet Rd, Bengaluru" || got[1].ID != "osm:way/2" {
		t.Errorf("way not mapped (should use its centre): %+v", got[1])
	}
}

func TestOverpass_FallsBackToNextMirror(t *testing.T) {
	badCalls := 0
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badCalls++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"elements":[{"type":"node","id":1,"lat":12.9750,"lon":77.6060,"tags":{"name":"Near Co","office":"it"}}]}`))
	}))
	defer good.Close()

	o := NewOverpass()
	o.Mirrors = []string{bad.URL, good.URL}

	got, err := o.Fetch(context.Background(), 12.9716, 77.5946, 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("Fetch() = %v, %v; want 1 place from the second mirror", got, err)
	}

	// The mirror that worked is tried first next time.
	if _, err := o.Fetch(context.Background(), 13.5, 77.5946, 10); err != nil {
		t.Fatal(err)
	}
	if badCalls != 1 {
		t.Errorf("failing mirror called %d times, want 1", badCalls)
	}
}

func TestOverpass_AllMirrorsFail(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer bad.Close()

	o := NewOverpass()
	o.Mirrors = []string{bad.URL, bad.URL}
	if _, err := o.Fetch(context.Background(), 12.97, 77.59, 10); err == nil {
		t.Error("Fetch() should fail when every mirror fails")
	}
}
