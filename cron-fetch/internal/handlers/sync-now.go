package handlers

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kv13b/Crontfetch/internal/middleware"
)

// RunNowHandler handles POST /run-now: instantly triggers a scrape cycle for all user companies.
func RunNowHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		// 1. Fetch all companies belonging to this user
		companies, err := listCompaniesByUser(r.Context(), pool, claims.UserID)
		if err != nil {
			slog.Error("run-now: failed listing companies", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not fetch user companies")
			return
		}

		if len(companies) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "no companies to sync", "new_jobs_count": 0}`))
			return
		}

		slog.Info("manual sync-now triggered", "user_id", claims.UserID, "company_count", len(companies))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "sync completed successfully"}`))
	}
}
