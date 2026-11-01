package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kv13b/Crontfetch/internal/fetcher"
	"github.com/kv13b/Crontfetch/internal/middleware"
)

// careerPageTimeout bounds the search; slow company websites are the usual
// reason it takes long.
const careerPageTimeout = 20 * time.Second

// FindCareerPage handles GET /career-page?name=&website=: suggests a career
// page for a company the user is about to track (the Track button on Jobs
// around me). website is optional. The suggestion is for the user to check,
// not saved. Can take up to 20 seconds.
func FindCareerPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		website := strings.TrimSpace(r.URL.Query().Get("website"))
		if name == "" || len(name) > 200 {
			writeError(w, http.StatusBadRequest, "name is required (up to 200 characters)")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), careerPageTimeout)
		defer cancel()
		page := fetcher.FindCareerPage(ctx, name, website)

		slog.Info("career page: suggested", "user_id", claims.UserID, "name", name, "found", page.Found, "platform", page.Platform, "url", page.URL)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(page)
	}
}
