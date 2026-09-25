package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/fetcher"
	"github.com/kv13b/Crontfetch/internal/middleware"
	"github.com/kv13b/Crontfetch/internal/models"
)

type createCompanyRequest struct {
	Name               string   `json:"name"`
	CareerURL          string   `json:"career_url"`
	Platform           string   `json:"platform"`
	Board              string   `json:"board"`
	MinExperienceYears *int     `json:"min_experience_years"`
	MaxExperienceYears *int     `json:"max_experience_years"`
	Roles              []string `json:"roles"`
	Locations          []string `json:"locations"`
}

// CreateCompany handles POST /companies: adds a career page for the
// authenticated user to track, along with the filters that decide which
// of its jobs are actually relevant to them.
func CreateCompany(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		var req createCompanyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		req.CareerURL = strings.TrimSpace(req.CareerURL)
		req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
		req.Board = strings.TrimSpace(req.Board)
		if req.Name == "" || req.CareerURL == "" {
			writeError(w, http.StatusBadRequest, "name and career_url are required")
			return
		}
		if !strings.HasPrefix(req.CareerURL, "http://") && !strings.HasPrefix(req.CareerURL, "https://") {
			writeError(w, http.StatusBadRequest, "career_url must start with http:// or https://")
			return
		}

		if req.Platform != "" && !fetcher.IsKnownPlatform(req.Platform) {
			writeError(w, http.StatusBadRequest, "platform must be one of: talentbrew, greenhouse, workday")
			return
		}
		if req.Platform == fetcher.PlatformWorkday {
			if detected, _ := fetcher.DetectPlatform(req.CareerURL); detected != fetcher.PlatformWorkday {
				writeError(w, http.StatusBadRequest, "career_url must be a *.myworkdayjobs.com link for workday")
				return
			}
		}
		if req.Board != "" && req.Platform == "" {
			writeError(w, http.StatusBadRequest, "platform is required when board is set")
			return
		}
		if req.Platform == fetcher.PlatformGreenhouse && req.Board == "" {
			if _, detected := fetcher.DetectPlatform(req.CareerURL); detected == "" {
				writeError(w, http.StatusBadRequest, "board is required for greenhouse unless career_url is a boards.greenhouse.io link")
				return
			}
		}

		if req.MinExperienceYears != nil && *req.MinExperienceYears < 0 {
			writeError(w, http.StatusBadRequest, "min_experience_years cannot be negative")
			return
		}
		if req.MaxExperienceYears != nil && *req.MaxExperienceYears < 0 {
			writeError(w, http.StatusBadRequest, "max_experience_years cannot be negative")
			return
		}
		if req.MinExperienceYears != nil && req.MaxExperienceYears != nil && *req.MinExperienceYears > *req.MaxExperienceYears {
			writeError(w, http.StatusBadRequest, "min_experience_years cannot be greater than max_experience_years")
			return
		}

		roles := cleanStrings(req.Roles)
		locations := cleanStrings(req.Locations)

		company, err := insertCompany(r.Context(), pool, newCompanyInput{
			UserID:             claims.UserID,
			Name:               req.Name,
			CareerURL:          req.CareerURL,
			Platform:           req.Platform,
			Board:              req.Board,
			MinExperienceYears: req.MinExperienceYears,
			MaxExperienceYears: req.MaxExperienceYears,
			Roles:              roles,
			Locations:          locations,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation {
				writeError(w, http.StatusConflict, "you're already tracking this career page")
				return
			}
			slog.Error("create company: inserting", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not save company")
			return
		}

		slog.Info("company: created", "company_id", company.ID, "user_id", claims.UserID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(company)
	}
}

// ListCompanies handles GET /companies: returns the authenticated user's
// tracked career pages.
func ListCompanies(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		companies, err := listCompaniesByUser(r.Context(), pool, claims.UserID)
		if err != nil {
			slog.Error("list companies: querying", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not list companies")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(companies)
	}
}

// cleanStrings trims whitespace from each entry and drops any that end up
// empty, so callers never have to deal with "" or "  " sneaking into a
// roles/locations filter.
func cleanStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			cleaned = append(cleaned, v)
		}
	}
	return cleaned
}

// companyColumns and scanCompany must stay in the same order.
const companyColumns = `id, user_id, name, career_url, platform, board, min_experience_years, max_experience_years, roles, locations, created_at, updated_at`

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanCompany(row rowScanner) (models.Company, error) {
	var c models.Company
	err := row.Scan(
		&c.ID, &c.UserID, &c.Name, &c.CareerURL, &c.Platform, &c.Board,
		&c.MinExperienceYears, &c.MaxExperienceYears, &c.Roles, &c.Locations,
		&c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

type newCompanyInput struct {
	UserID             string
	Name               string
	CareerURL          string
	Platform           string
	Board              string
	MinExperienceYears *int
	MaxExperienceYears *int
	Roles              []string
	Locations          []string
}

func insertCompany(ctx context.Context, pool *pgxpool.Pool, in newCompanyInput) (models.Company, error) {
	const query = `
		INSERT INTO companies (user_id, name, career_url, platform, board, min_experience_years, max_experience_years, roles, locations)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + companyColumns

	return scanCompany(pool.QueryRow(ctx, query,
		in.UserID, in.Name, in.CareerURL, in.Platform, in.Board,
		in.MinExperienceYears, in.MaxExperienceYears, in.Roles, in.Locations,
	))
}

func listCompaniesByUser(ctx context.Context, pool *pgxpool.Pool, userID string) ([]models.Company, error) {
	const query = `
		SELECT ` + companyColumns + `
		FROM companies
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	companies := make([]models.Company, 0)
	for rows.Next() {
		c, err := scanCompany(rows)
		if err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, rows.Err()
}
