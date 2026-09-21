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

// Me handles GET /me: returns the authenticated caller's own profile.
// It must be mounted behind middleware.Auth, which is what populates the
// claims this handler reads from the request context.
func Me(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		user, err := getUserByID(r.Context(), pool, claims.UserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			slog.Error("me: looking up user", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not look up user")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

func getUserByID(ctx context.Context, pool *pgxpool.Pool, id string) (models.User, error) {
	const query = `
		SELECT id, name, email, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt,
	)
	return user, err
}
