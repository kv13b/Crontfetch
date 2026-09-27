package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/fetcher"
	"github.com/kv13b/Crontfetch/internal/middleware"
	"github.com/kv13b/Crontfetch/internal/models"
)

// detectTimeout bounds platform detection so a slow career site can't hold
// up saving a company.
const detectTimeout = 20 * time.Second

// companyFields is the user-editable part of a company. Create and update
// both go through it so they apply exactly the same rules.
type companyFields struct {
	Name               string
	CareerURL          string
	Platform           string
	Board              string
	MinExperienceYears *int
	MaxExperienceYears *int
	Roles              []string
	Locations          []string
}

// validateCompany tidies f in place and checks it, returning a message for a
// 400 response or "" if it's valid. With no platform set, it also detects
// one from career_url and fills in platform and board when the result is usable.
func validateCompany(ctx context.Context, f *companyFields) string {
	f.Name = strings.TrimSpace(f.Name)
	f.CareerURL = strings.TrimSpace(f.CareerURL)
	f.Platform = strings.ToLower(strings.TrimSpace(f.Platform))
	f.Board = strings.TrimSpace(f.Board)

	if f.Name == "" || f.CareerURL == "" {
		return "name and career_url are required"
	}
	if !strings.HasPrefix(f.CareerURL, "http://") && !strings.HasPrefix(f.CareerURL, "https://") {
		return "career_url must start with http:// or https://"
	}

	if f.Platform != "" && !fetcher.IsKnownPlatform(f.Platform) {
		return "platform must be one of: talentbrew, greenhouse, workday, lever, ashby, smartrecruiters, workable, recruitee, beesite"
	}
	if f.Platform == fetcher.PlatformWorkday {
		if detected, _ := fetcher.DetectPlatform(f.CareerURL); detected != fetcher.PlatformWorkday {
			return "career_url must be a *.myworkdayjobs.com link for workday"
		}
	}
	if f.Board != "" && f.Platform == "" {
		return "platform is required when board is set"
	}
	if f.Platform == fetcher.PlatformBeeSite && f.Board != "" {
		if u, err := url.Parse(f.Board); err != nil || u.Scheme != "https" || u.Host == "" {
			return "board must be the BeeSite API address, e.g. https://jobs.api.example.com"
		}
	}
	if fetcher.NeedsBoard(f.Platform) && f.Board == "" {
		if p, b := fetcher.DetectPlatform(f.CareerURL); p != f.Platform || b == "" {
			return "board is required for " + f.Platform + " unless career_url is that platform's own job board link"
		}
	}

	if f.MinExperienceYears != nil && *f.MinExperienceYears < 0 {
		return "min_experience_years cannot be negative"
	}
	if f.MaxExperienceYears != nil && *f.MaxExperienceYears < 0 {
		return "max_experience_years cannot be negative"
	}
	if f.MinExperienceYears != nil && f.MaxExperienceYears != nil && *f.MinExperienceYears > *f.MaxExperienceYears {
		return "min_experience_years cannot be greater than max_experience_years"
	}

	// Work out the platform now and save it, so later job fetches don't
	// repeat the detection. If nothing usable is found, save the company
	// anyway; the jobs endpoint will say why it can't be read.
	if f.Platform == "" {
		detectCtx, cancel := context.WithTimeout(ctx, detectTimeout)
		detected := fetcher.Detect(detectCtx, f.CareerURL)
		cancel()
		slog.Info("company: platform detection", "career_url", f.CareerURL, "platform", detected.Platform, "board", detected.Board)
		if detected.Usable() {
			f.Platform, f.Board = detected.Platform, detected.Board
		}
	}

	f.Roles = cleanStrings(f.Roles)
	f.Locations = cleanStrings(f.Locations)
	return ""
}

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

		f := companyFields{
			Name:               req.Name,
			CareerURL:          req.CareerURL,
			Platform:           req.Platform,
			Board:              req.Board,
			MinExperienceYears: req.MinExperienceYears,
			MaxExperienceYears: req.MaxExperienceYears,
			Roles:              req.Roles,
			Locations:          req.Locations,
		}
		if msg := validateCompany(r.Context(), &f); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}

		company, err := insertCompany(r.Context(), pool, claims.UserID, f)
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

// optionalInt tells "field absent" (leave unchanged) apart from an explicit
// null (clear it), which a plain *int can't.
type optionalInt struct {
	Set   bool
	Value *int
}

func (o *optionalInt) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// updateCompanyRequest is a partial update: only the fields present in the
// JSON body are changed.
type updateCompanyRequest struct {
	Name               *string     `json:"name"`
	CareerURL          *string     `json:"career_url"`
	Platform           *string     `json:"platform"`
	Board              *string     `json:"board"`
	MinExperienceYears optionalInt `json:"min_experience_years"`
	MaxExperienceYears optionalInt `json:"max_experience_years"`
	Roles              []string    `json:"roles"`
	Locations          []string    `json:"locations"`
}

func (u updateCompanyRequest) isEmpty() bool {
	return u.Name == nil && u.CareerURL == nil && u.Platform == nil && u.Board == nil &&
		!u.MinExperienceYears.Set && !u.MaxExperienceYears.Set && u.Roles == nil && u.Locations == nil
}

// applyCompanyPatch returns the company's fields with the patch applied.
//
// Platform and board describe career_url, so they're reset (and later
// re-detected) when the URL changes without new ones being supplied, and the
// board is dropped when the platform changes without a new board.
func applyCompanyPatch(c models.Company, p updateCompanyRequest) companyFields {
	f := companyFields{
		Name:               c.Name,
		CareerURL:          c.CareerURL,
		Platform:           c.Platform,
		Board:              c.Board,
		MinExperienceYears: c.MinExperienceYears,
		MaxExperienceYears: c.MaxExperienceYears,
		Roles:              c.Roles,
		Locations:          c.Locations,
	}

	if p.Name != nil {
		f.Name = *p.Name
	}
	if p.CareerURL != nil {
		f.CareerURL = *p.CareerURL
		if strings.TrimSpace(*p.CareerURL) != c.CareerURL && p.Platform == nil && p.Board == nil {
			f.Platform, f.Board = "", ""
		}
	}
	if p.Platform != nil {
		f.Platform = *p.Platform
		if strings.ToLower(strings.TrimSpace(*p.Platform)) != c.Platform && p.Board == nil {
			f.Board = ""
		}
	}
	if p.Board != nil {
		f.Board = *p.Board
	}
	if p.MinExperienceYears.Set {
		f.MinExperienceYears = p.MinExperienceYears.Value
	}
	if p.MaxExperienceYears.Set {
		f.MaxExperienceYears = p.MaxExperienceYears.Value
	}
	if p.Roles != nil {
		f.Roles = p.Roles
	}
	if p.Locations != nil {
		f.Locations = p.Locations
	}
	return f
}

// isCompanyNotFound reports whether a lookup failed because no such row
// exists, or because the id in the URL isn't even a valid UUID.
func isCompanyNotFound(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == postgresInvalidTextRepresentation)
}

// UpdateCompany handles PATCH /companies/{id}: changes only the fields sent,
// then re-checks the whole company with the same rules as creating one.
func UpdateCompany(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		// Unknown fields are rejected so a typo like "min_exp" fails loudly
		// instead of silently changing nothing.
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req updateCompanyRequest
		if err := dec.Decode(&req); err != nil {
			if strings.HasPrefix(err.Error(), "json: unknown field") {
				writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.isEmpty() {
			writeError(w, http.StatusBadRequest, "no fields to update")
			return
		}

		existing, err := getCompanyByID(r.Context(), pool, claims.UserID, r.PathValue("id"))
		if err != nil {
			if isCompanyNotFound(err) {
				writeError(w, http.StatusNotFound, "company not found")
				return
			}
			slog.Error("update company: looking up company", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not look up company")
			return
		}

		f := applyCompanyPatch(existing, req)
		if msg := validateCompany(r.Context(), &f); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}

		updated, err := updateCompany(r.Context(), pool, claims.UserID, existing.ID, f)
		if err != nil {
			var pgErr *pgconn.PgError
			switch {
			case errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation:
				writeError(w, http.StatusConflict, "you're already tracking this career page")
			case errors.Is(err, pgx.ErrNoRows):
				writeError(w, http.StatusNotFound, "company not found")
			default:
				slog.Error("update company: updating", "error", err, "company_id", existing.ID, "user_id", claims.UserID)
				writeError(w, http.StatusInternalServerError, "could not update company")
			}
			return
		}

		slog.Info("company: updated", "company_id", updated.ID, "user_id", claims.UserID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
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

func insertCompany(ctx context.Context, pool *pgxpool.Pool, userID string, f companyFields) (models.Company, error) {
	const query = `
		INSERT INTO companies (user_id, name, career_url, platform, board, min_experience_years, max_experience_years, roles, locations)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + companyColumns

	return scanCompany(pool.QueryRow(ctx, query,
		userID, f.Name, f.CareerURL, f.Platform, f.Board,
		f.MinExperienceYears, f.MaxExperienceYears, f.Roles, f.Locations,
	))
}

// updateCompany writes every editable column, scoped to the owner.
func updateCompany(ctx context.Context, pool *pgxpool.Pool, userID, id string, f companyFields) (models.Company, error) {
	const query = `
		UPDATE companies
		SET name = $3, career_url = $4, platform = $5, board = $6,
		    min_experience_years = $7, max_experience_years = $8,
		    roles = $9, locations = $10, updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING ` + companyColumns

	return scanCompany(pool.QueryRow(ctx, query,
		id, userID, f.Name, f.CareerURL, f.Platform, f.Board,
		f.MinExperienceYears, f.MaxExperienceYears, f.Roles, f.Locations,
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
