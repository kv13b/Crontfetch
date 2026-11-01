package geo

import (
	"context"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DBSource reads offices imported into the osm_places table by
// cmd/importplaces. It's instant and doesn't depend on any public server,
// so it's tried first; areas the import doesn't cover fall through to the
// live sources.
type DBSource struct {
	Pool *pgxpool.Pool
}

func (d *DBSource) Name() string { return "database" }

// Uncached: the table is indexed and cheap to query, and skipping the cache
// means a fresh import shows up without restarting the server.
func (d *DBSource) Uncached() bool { return true }

// Fetch returns the imported offices in the bounding box around the point,
// or ErrNoData if there are none.
func (d *DBSource) Fetch(ctx context.Context, lat, lng, radiusKm float64) ([]OSMPlace, error) {
	const query = `
		SELECT id, name, kind, website, address, lat, lng
		FROM osm_places
		WHERE lat BETWEEN $1 AND $2 AND lng BETWEEN $3 AND $4
	`

	dLat := radiusKm / 111.0
	dLng := radiusKm / (111.0 * math.Cos(lat*math.Pi/180))
	rows, err := d.Pool.Query(ctx, query, lat-dLat, lat+dLat, lng-dLng, lng+dLng)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	places := make([]OSMPlace, 0)
	for rows.Next() {
		var p OSMPlace
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.Website, &p.Address, &p.Lat, &p.Lng); err != nil {
			return nil, err
		}
		places = append(places, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(places) == 0 {
		return nil, ErrNoData
	}
	return places, nil
}
