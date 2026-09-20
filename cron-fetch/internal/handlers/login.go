package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/kv13b/Crontfetch/internal/jwt"
	"github.com/kv13b/Crontfetch/internal/models"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /login: it verifies credentials and returns a JWT.
func Login(pool *pgxpool.Pool, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.Email == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		user, passwordHash, err := getUserByEmail(r.Context(), pool, req.Email)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, "invalid email or password")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not look up user")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		token, err := jwt.GenerateToken(user.ID, user.Email, jwtSecret, tokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not generate token")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(authResponse{Token: token, User: user})
	}
}

func getUserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (models.User, string, error) {
	const query = `
		SELECT id, name, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user models.User
	var passwordHash string
	err := pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Name, &user.Email, &passwordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	return user, passwordHash, err
}
