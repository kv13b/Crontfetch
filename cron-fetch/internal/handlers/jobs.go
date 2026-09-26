package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/fetcher"
	"github.com/kv13b/Crontfetch/internal/middleware"
	"github.com/kv13b/Crontfetch/internal/models"
)

// maxPagesPerFetch bounds how many result pages (15 jobs each) one request
// will read from a career site.
const maxPagesPerFetch = 100

// postgresInvalidTextRepresentation is returned when a value can't be
// parsed as the column's type, e.g. a malformed UUID in a URL.
const postgresInvalidTextRepresentation = "22P02"

type companyJobsResponse struct {
	Company      string        `json:"company"`
	TotalScanned int           `json:"total_scanned"`
	TotalMatched int           `json:"total_matched"`
	Warnings     []string      `json:"warnings,omitempty"`
	Jobs         []fetcher.Job `json:"jobs"`
}

// CompanyJobs handles GET /companies/{id}/jobs: fetches the company's
// current job listings and returns only those matching its saved filters.
func CompanyJobs(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		company, err := getCompanyByID(r.Context(), pool, claims.UserID, r.PathValue("id"))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == postgresInvalidTextRepresentation) {
				writeError(w, http.StatusNotFound, "company not found")
				return
			}
			slog.Error("company jobs: looking up company", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not look up company")
			return
		}

		jobs, err := fetcher.FetchJobs(r.Context(), company.CareerURL, company.Platform, company.Board, maxPagesPerFetch)
		if err != nil {
			var unsupported *fetcher.UnsupportedPlatformError
			switch {
			case errors.As(err, &unsupported):
				slog.Warn("company jobs: unsupported platform", "platform", unsupported.Platform, "career_url", company.CareerURL)
				writeError(w, http.StatusUnprocessableEntity, "this career site uses "+unsupported.Platform+", which isn't supported yet — only TalentBrew, Greenhouse and Workday can be read right now")
				return
			case errors.Is(err, fetcher.ErrUnsupportedPlatform):
				slog.Warn("company jobs: unrecognised platform", "career_url", company.CareerURL)
				writeError(w, http.StatusUnprocessableEntity, "couldn't recognise this career site's platform — only TalentBrew, Greenhouse and Workday can be read right now")
				return
			case errors.Is(err, fetcher.ErrWorkdaySiteNotFound):
				writeError(w, http.StatusUnprocessableEntity, "no workday career site found at the saved career_url")
				return
			case errors.Is(err, fetcher.ErrMissingBoard):
				writeError(w, http.StatusUnprocessableEntity, "this site uses Greenhouse, but its board name couldn't be worked out — save the company with platform \"greenhouse\" and its board name")
				return
			case errors.Is(err, fetcher.ErrBoardNotFound):
				writeError(w, http.StatusUnprocessableEntity, "no greenhouse job board found with the saved board name")
				return
			}
			slog.Error("company jobs: fetching", "error", err, "company_id", company.ID, "career_url", company.CareerURL)
			writeError(w, http.StatusBadGateway, "could not fetch jobs from the career page")
			return
		}

		matched := fetcher.FilterJobs(jobs, company.Roles, company.Locations)
		slog.Info("company jobs: fetched", "company_id", company.ID, "scanned", len(jobs), "matched", len(matched))

		resp := companyJobsResponse{
			Company:      company.Name,
			TotalScanned: len(jobs),
			TotalMatched: len(matched),
			Jobs:         matched,
		}
		if company.MinExperienceYears != nil || company.MaxExperienceYears != nil {
			resp.Warnings = append(resp.Warnings, "experience filters are saved but not applied yet: job listings don't include required experience")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// getCompanyByID scopes the lookup to userID, so asking for another user's
// company looks exactly like asking for one that doesn't exist.
func getCompanyByID(ctx context.Context, pool *pgxpool.Pool, userID, id string) (models.Company, error) {
	const query = `
		SELECT ` + companyColumns + `
		FROM companies
		WHERE id = $1 AND user_id = $2
	`

	return scanCompany(pool.QueryRow(ctx, query, id, userID))
}
