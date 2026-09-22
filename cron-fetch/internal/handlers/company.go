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

	"github.com/kv13b/Crontfetch/internal/middleware"
	"github.com/kv13b/Crontfetch/internal/models"
)

type createCompanyRequest struct {
	Name      string `json:"name"`
	CareerURL string `json:"career_url"`
}

// CreateCompany handles POST /companies: adds a career page for the
// authenticated user to track.
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
		if req.Name == "" || req.CareerURL == "" {
			writeError(w, http.StatusBadRequest, "name and career_url are required")
			return
		}
		if !strings.HasPrefix(req.CareerURL, "http://") && !strings.HasPrefix(req.CareerURL, "https://") {
			writeError(w, http.StatusBadRequest, "career_url must start with http:// or https://")
			return
		}

		company, err := insertCompany(r.Context(), pool, claims.UserID, req.Name, req.CareerURL)
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

func insertCompany(ctx context.Context, pool *pgxpool.Pool, userID, name, careerURL string) (models.Company, error) {
	const query = `
		INSERT INTO companies (user_id, name, career_url)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, name, career_url, created_at, updated_at
	`

	var c models.Company
	err := pool.QueryRow(ctx, query, userID, name, careerURL).Scan(
		&c.ID, &c.UserID, &c.Name, &c.CareerURL, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

func listCompaniesByUser(ctx context.Context, pool *pgxpool.Pool, userID string) ([]models.Company, error) {
	const query = `
		SELECT id, user_id, name, career_url, created_at, updated_at
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
		var c models.Company
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.CareerURL, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, rows.Err()
}
