//go:build !cgo

// Command importplaces loads named offices from an OpenStreetMap .osm.pbf
// extract into the osm_places table, so "companies around me" is served from
// our own database instead of public map servers.
//
//	CGO_ENABLED=0 go run ./cmd/importplaces -file karnataka.osm.pbf
//
// It builds only with cgo off, so the PBF reader uses Go's own zlib rather
// than a C one that needs a working C toolchain (in PowerShell:
// $env:CGO_ENABLED="0"; go run ./cmd/importplaces -file ...).
//
// Extracts: https://download.openstreetmap.fr/extracts/asia/india/ (per
// state; karnataka.osm.pbf covers Bangalore) or https://download.geofabrik.de/.
// Re-running replaces the previous import.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"

	"github.com/kv13b/Crontfetch/internal/db"
	"github.com/kv13b/Crontfetch/internal/geo"
)

type bbox struct{ minLat, minLng, maxLat, maxLng float64 }

func (b *bbox) contains(lat, lng float64) bool {
	return b == nil || (lat >= b.minLat && lat <= b.maxLat && lng >= b.minLng && lng <= b.maxLng)
}

func main() {
	file := flag.String("file", "", "path to an .osm.pbf extract (required)")
	box := flag.String("bbox", "", "optional minLat,minLng,maxLat,maxLng to import only that area, e.g. 12.7,77.3,13.3,77.9 for Bangalore")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: importplaces -file <extract.osm.pbf> [-bbox minLat,minLng,maxLat,maxLng]")
		os.Exit(2)
	}
	area, err := parseBBox(*box)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx := context.Background()
	start := time.Now()

	places, err := readOffices(ctx, *file, area)
	if err != nil {
		slog.Error("reading extract", "error", err)
		os.Exit(1)
	}
	slog.Info("read offices from extract", "count", len(places), "took", time.Since(start).Round(time.Second))

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		slog.Error("connecting to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	n, err := save(ctx, pool, places, area)
	if err != nil {
		slog.Error("saving offices", "error", err)
		os.Exit(1)
	}
	slog.Info("import done", "saved", n, "took", time.Since(start).Round(time.Second))
}

func parseBBox(s string) (*bbox, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("-bbox must be minLat,minLng,maxLat,maxLng")
	}
	var v [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("-bbox: %q is not a number", p)
		}
		v[i] = f
	}
	b := &bbox{v[0], v[1], v[2], v[3]}
	if b.minLat >= b.maxLat || b.minLng >= b.maxLng || !geo.ValidCoords(b.minLat, b.minLng) || !geo.ValidCoords(b.maxLat, b.maxLng) {
		return nil, fmt.Errorf("-bbox is out of range or the wrong way round")
	}
	return b, nil
}

// readOffices makes two passes over the extract. Ways (building outlines)
// only list node IDs, and nodes come before ways in the file, so the first
// pass collects office nodes and office ways, and the second looks up the
// coordinates of the ways' nodes to place each way at its centre.
func readOffices(ctx context.Context, path string, area *bbox) ([]geo.OSMPlace, error) {
	var places []geo.OSMPlace
	type officeWay struct {
		id   osm.WayID
		tags map[string]string
		refs []osm.NodeID
	}
	var ways []officeWay
	needed := make(map[osm.NodeID]bool)

	err := scan(ctx, path, func(s *osmpbf.Scanner) {
		s.SkipRelations = true
		s.FilterNode = func(n *osm.Node) bool { return isOfficeTags(n.Tags) }
		s.FilterWay = func(w *osm.Way) bool { return isOfficeTags(w.Tags) }
	}, func(o osm.Object) {
		switch e := o.(type) {
		case *osm.Node:
			if area.contains(e.Lat, e.Lon) {
				places = append(places, geo.PlaceFromTags(fmt.Sprintf("osm:node/%d", e.ID), e.Tags.Map(), e.Lat, e.Lon))
			}
		case *osm.Way:
			w := officeWay{id: e.ID, tags: e.Tags.Map()}
			for _, wn := range e.Nodes {
				w.refs = append(w.refs, wn.ID)
				needed[wn.ID] = true
			}
			ways = append(ways, w)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("first pass: %w", err)
	}
	slog.Info("first pass done", "office_nodes", len(places), "office_ways", len(ways))

	type point struct{ lat, lng float64 }
	coords := make(map[osm.NodeID]point, len(needed))
	err = scan(ctx, path, func(s *osmpbf.Scanner) {
		s.SkipWays, s.SkipRelations = true, true
		// needed is only read here, so the concurrent decoders can share it.
		s.FilterNode = func(n *osm.Node) bool { return needed[n.ID] }
	}, func(o osm.Object) {
		if n, ok := o.(*osm.Node); ok {
			coords[n.ID] = point{n.Lat, n.Lon}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("second pass: %w", err)
	}

	for _, w := range ways {
		var sumLat, sumLng float64
		count := 0
		for _, ref := range w.refs {
			if p, ok := coords[ref]; ok {
				sumLat, sumLng = sumLat+p.lat, sumLng+p.lng
				count++
			}
		}
		if count == 0 {
			continue
		}
		lat, lng := sumLat/float64(count), sumLng/float64(count)
		if area.contains(lat, lng) {
			places = append(places, geo.PlaceFromTags(fmt.Sprintf("osm:way/%d", w.id), w.tags, lat, lng))
		}
	}
	return places, nil
}

// isOfficeTags checks the cheap case (no office tag) before building a map.
func isOfficeTags(tags osm.Tags) bool {
	if len(tags) == 0 || tags.Find("office") == "" {
		return false
	}
	return geo.IsOffice(tags.Map())
}

func scan(ctx context.Context, path string, configure func(*osmpbf.Scanner), each func(osm.Object)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	s := osmpbf.New(ctx, f, runtime.GOMAXPROCS(-1))
	defer s.Close()
	configure(s)
	for s.Scan() {
		each(s.Object())
	}
	return s.Err()
}

// save replaces the previous import (within the bbox, if one was given) in
// a single transaction, so the table is never half-empty.
func save(ctx context.Context, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, places []geo.OSMPlace, area *bbox) (int64, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if area == nil {
		_, err = tx.Exec(ctx, `DELETE FROM osm_places`)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM osm_places WHERE lat BETWEEN $1 AND $2 AND lng BETWEEN $3 AND $4`,
			area.minLat, area.maxLat, area.minLng, area.maxLng)
	}
	if err != nil {
		return 0, fmt.Errorf("clearing previous import: %w", err)
	}

	rows := make([][]any, 0, len(places))
	seen := make(map[string]bool, len(places))
	for _, p := range places {
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		rows = append(rows, []any{p.ID, p.Name, p.Kind, p.Website, p.Address, p.Lat, p.Lng})
	}
	n, err := tx.CopyFrom(ctx, pgx.Identifier{"osm_places"},
		[]string{"id", "name", "kind", "website", "address", "lat", "lng"}, pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("copying rows: %w", err)
	}
	return n, tx.Commit(ctx)
}
