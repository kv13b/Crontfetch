package geo

import (
	"context"
	"errors"
	"testing"
)

type fakeSource struct {
	name   string
	places []OSMPlace
	err    error
	calls  int
}

func (f *fakeSource) Name() string { return f.name }

func (f *fakeSource) Fetch(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, error) {
	f.calls++
	// Return a copy: Places dedupes in place.
	return append([]OSMPlace(nil), f.places...), f.err
}

func TestPlaces_FiltersSortsAndCaches(t *testing.T) {
	src := &fakeSource{name: "fake", places: []OSMPlace{
		{ID: "osm:node/1", Name: "Medium", Lat: 12.9352, Lng: 77.6245}, // ~5 km
		{ID: "osm:node/2", Name: "Close", Lat: 12.9750, Lng: 77.6060},  // ~1.4 km
		{ID: "osm:node/3", Name: "Far", Lat: 13.2, Lng: 77.9},          // outside 10 km
	}}
	p := NewPlaces(src)

	got, err := p.Offices(context.Background(), 12.9716, 77.5946, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "Close" || got[1].Name != "Medium" {
		t.Fatalf("Offices() = %+v, want Close then Medium", got)
	}
	if got[0].DistanceKm <= 0 || got[0].Source != "fake" {
		t.Errorf("distance/source not set: %+v", got[0])
	}

	// A short move stays in the same grid cell and is served from cache.
	if _, err := p.Offices(context.Background(), 12.9730, 77.5960, 10); err != nil {
		t.Fatal(err)
	}
	if src.calls != 1 {
		t.Errorf("source called %d times, want 1", src.calls)
	}
}

func TestPlaces_FallsBackToNextSource(t *testing.T) {
	down := &fakeSource{name: "overpass", err: errors.New("status 504")}
	up := &fakeSource{name: "photon", places: []OSMPlace{{ID: "osm:node/1", Name: "Cisco", Lat: 12.9750, Lng: 77.6060}}}
	p := NewPlaces(down, up)

	got, err := p.Offices(context.Background(), 12.9716, 77.5946, 10)
	if err != nil || len(got) != 1 || got[0].Source != "photon" {
		t.Fatalf("Offices() = %+v, %v; want Cisco from photon", got, err)
	}
}

func TestPlaces_AllSourcesFail(t *testing.T) {
	p := NewPlaces(&fakeSource{name: "a", err: errors.New("down")}, &fakeSource{name: "b", err: errors.New("down")})
	if _, err := p.Offices(context.Background(), 12.97, 77.59, 10); err == nil {
		t.Error("Offices() should fail when every source fails")
	}
}

func TestDedupe(t *testing.T) {
	got := dedupe([]OSMPlace{
		{ID: "osm:node/1", Name: "SAP", Lat: 12.97501, Lng: 77.60601},
		{ID: "osm:way/2", Name: "SAP ", Lat: 12.97502, Lng: 77.60602}, // same office, building outline
		{ID: "osm:node/1", Name: "SAP Labs", Lat: 12.9, Lng: 77.6},    // same ID from another search
		{ID: "osm:node/3", Name: "", Lat: 12.9, Lng: 77.6},            // unnamed
		{ID: "osm:node/4", Name: "Intel", Lat: 12.95, Lng: 77.70},
	})
	if len(got) != 2 || got[0].Name != "SAP" || got[1].Name != "Intel" {
		t.Errorf("dedupe() = %+v, want SAP and Intel", got)
	}
}

func TestPlaces_SkipsFailedSourceDuringCooldown(t *testing.T) {
	down := &fakeSource{name: "overpass", err: errors.New("status 504")}
	up := &fakeSource{name: "photon", places: []OSMPlace{{ID: "osm:node/1", Name: "Cisco", Lat: 12.9750, Lng: 77.6060}}}
	p := NewPlaces(down, up)

	if _, err := p.Offices(context.Background(), 12.9716, 77.5946, 10); err != nil {
		t.Fatal(err)
	}
	// A different area misses the cache; the failed source is skipped.
	if _, err := p.Offices(context.Background(), 13.5, 77.5946, 10); err != nil {
		t.Fatal(err)
	}
	if down.calls != 1 || up.calls != 2 {
		t.Errorf("calls: overpass %d (want 1), photon %d (want 2)", down.calls, up.calls)
	}
}

func TestPlaces_NoDataFallsThroughWithoutCooldown(t *testing.T) {
	db := &fakeSource{name: "database", err: ErrNoData}
	live := &fakeSource{name: "overpass", places: []OSMPlace{{ID: "osm:node/1", Name: "Cisco", Lat: 12.9750, Lng: 77.6060}}}
	p := NewPlaces(db, live)

	got, err := p.Offices(context.Background(), 12.9716, 77.5946, 10)
	if err != nil || len(got) != 1 || got[0].Source != "overpass" {
		t.Fatalf("Offices() = %+v, %v; want Cisco from overpass", got, err)
	}
	// "No data here" isn't a failure: the database is still asked for the next area.
	if _, err := p.Offices(context.Background(), 13.5, 77.5946, 10); err != nil {
		t.Fatal(err)
	}
	if db.calls != 2 {
		t.Errorf("database asked %d times, want 2 (no cooldown for ErrNoData)", db.calls)
	}
}

type uncachedSource struct{ fakeSource }

func (u *uncachedSource) Uncached() bool { return true }

func TestPlaces_UncachedSourceIsAskedEveryTime(t *testing.T) {
	db := &uncachedSource{fakeSource{name: "database", places: []OSMPlace{{ID: "osm:node/1", Name: "Cisco", Lat: 12.9750, Lng: 77.6060}}}}
	p := NewPlaces(db)
	for range 2 {
		if _, err := p.Offices(context.Background(), 12.9716, 77.5946, 10); err != nil {
			t.Fatal(err)
		}
	}
	if db.calls != 2 {
		t.Errorf("database asked %d times, want 2 (its results aren't cached)", db.calls)
	}
}
