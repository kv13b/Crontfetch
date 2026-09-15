// Package handlers holds the HTTP handlers for the CronFetch API.
package handlers

import (
	"encoding/json"
	"net/http"
)

// Health is a liveness probe, mirroring the old Flask GET /health.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "online"})
}
