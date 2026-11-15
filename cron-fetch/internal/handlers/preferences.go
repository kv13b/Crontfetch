package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/middleware"
	"github.com/kv13b/Crontfetch/internal/models"
)

// validatePreferences tidies p in place and checks it, returning a message
// for a 400 response or "" if it's valid.
func validatePreferences(p *models.Preferences) string {
	if p.MinExperienceYears != nil && *p.MinExperienceYears < 0 {
		return "min_experience_years cannot be negative"
	}
	if p.MaxExperienceYears != nil && *p.MaxExperienceYears < 0 {
		return "max_experience_years cannot be negative"
	}
	if p.MinExperienceYears != nil && p.MaxExperienceYears != nil && *p.MinExperienceYears > *p.MaxExperienceYears {
		return "min_experience_years cannot be greater than max_experience_years"
	}

	p.Locations = cleanStrings(p.Locations)
	p.Roles = cleanStrings(p.Roles)
	p.Interests = cleanStrings(p.Interests)
	return ""
}

// GetPreferences handles GET /me/preferences: returns what the caller is
// looking for, used to fill the job filters.
func GetPreferences(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		p, err := getPreferences(r.Context(), pool, claims.UserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			slog.Error("preferences: looking up", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not look up preferences")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

// UpdatePreferences handles PUT /me/preferences: replaces all of the
// caller's preferences. Omitted fields are cleared.
func UpdatePreferences(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		var p models.Preferences
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if msg := validatePreferences(&p); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}

		saved, err := savePreferences(r.Context(), pool, claims.UserID, p)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			slog.Error("preferences: saving", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not save preferences")
			return
		}

		slog.Info("preferences: saved", "user_id", claims.UserID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(saved)
	}
}

func getPreferences(ctx context.Context, pool *pgxpool.Pool, userID string) (models.Preferences, error) {
	const query = `
		SELECT locations, roles, interests, min_experience_years, max_experience_years
		FROM users
		WHERE id = $1
	`
	var p models.Preferences
	err := pool.QueryRow(ctx, query, userID).Scan(
		&p.Locations, &p.Roles, &p.Interests, &p.MinExperienceYears, &p.MaxExperienceYears,
	)
	return p, err
}

func savePreferences(ctx context.Context, pool *pgxpool.Pool, userID string, p models.Preferences) (models.Preferences, error) {
	const query = `
		UPDATE users
		SET locations = $2, roles = $3, interests = $4,
		    min_experience_years = $5, max_experience_years = $6, updated_at = now()
		WHERE id = $1
		RETURNING locations, roles, interests, min_experience_years, max_experience_years
	`
	var saved models.Preferences
	err := pool.QueryRow(ctx, query, userID, p.Locations, p.Roles, p.Interests, p.MinExperienceYears, p.MaxExperienceYears).Scan(
		&saved.Locations, &saved.Roles, &saved.Interests, &saved.MinExperienceYears, &saved.MaxExperienceYears,
	)
	return saved, err
}
