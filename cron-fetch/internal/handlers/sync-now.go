package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/jobsync"
	"github.com/kv13b/Crontfetch/internal/middleware"
)

// runNowCooldown is how long a user waits between manual syncs, so repeated
// clicks can't hammer the career sites.
const runNowCooldown = time.Minute

type runNowFailure struct {
	CompanyID string `json:"company_id"`
	Name      string `json:"name"`
	Error     string `json:"error"`
}

type runNowResponse struct {
	Companies int             `json:"companies"`
	NewJobs   int             `json:"new_jobs"`
	Failed    []runNowFailure `json:"failed"`
}

// cooldown remembers when each user last started a manual sync.
// shortcut: in memory, so it resets on restart and isn't shared between server instances; move to the database if the API ever runs more than one instance.
type cooldown struct {
	mu   sync.Mutex
	last map[string]time.Time
}

// allow reports whether userID may start a sync at now, and records it if so.
func (c *cooldown) allow(userID string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.last[userID]; ok && now.Sub(t) < runNowCooldown {
		return false
	}
	c.last[userID] = now
	return true
}

// RunNowHandler handles POST /run-now: checks all of the caller's companies
// for new matching jobs right away, the same way the background sync does,
// including Telegram alerts. It runs the companies one after another, so it
// can take a while; one company's failure doesn't stop the rest.
func RunNowHandler(pool *pgxpool.Pool, botToken, chatID string) http.HandlerFunc {
	limiter := &cooldown{last: make(map[string]time.Time)}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		// Without Telegram, new jobs would be recorded as seen and their
		// alerts lost for good, so refuse instead.
		if botToken == "" || chatID == "" {
			writeError(w, http.StatusServiceUnavailable, "job alerts are not configured on the server")
			return
		}

		if !limiter.allow(claims.UserID, time.Now()) {
			writeError(w, http.StatusTooManyRequests, "please wait a minute before syncing again")
			return
		}

		companies, err := listCompaniesByUser(r.Context(), pool, claims.UserID)
		if err != nil {
			slog.Error("run-now: listing companies", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not fetch user companies")
			return
		}

		slog.Info("run-now: started", "user_id", claims.UserID, "companies", len(companies))
		resp := runNowResponse{Companies: len(companies), Failed: []runNowFailure{}}
		for _, c := range companies {
			n, err := jobsync.SyncCompany(r.Context(), pool, botToken, chatID, c)
			if err != nil {
				slog.Error("run-now: syncing company", "error", err, "company_id", c.ID, "user_id", claims.UserID)
				resp.Failed = append(resp.Failed, runNowFailure{CompanyID: c.ID, Name: c.Name, Error: err.Error()})
				continue
			}
			resp.NewJobs += n
		}
		slog.Info("run-now: done", "user_id", claims.UserID, "new_jobs", resp.NewJobs, "failed", len(resp.Failed))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
