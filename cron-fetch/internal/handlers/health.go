// Package handlers holds the HTTP handlers for the CronFetch API.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Health is a liveness probe, mirroring the old Flask GET /health.
// It also pings the database so callers can see whether the DB is reachable.
func Health(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "online"
		if err := pool.Ping(r.Context()); err != nil {
			dbStatus = "unreachable"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "online",
			"db":     dbStatus,
		})
	}
}
