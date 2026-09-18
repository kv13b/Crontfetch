package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/kv13b/Crontfetch/internal/jwt"
	"github.com/kv13b/Crontfetch/internal/models"
)

// tokenTTL is how long an issued JWT stays valid.
const tokenTTL = 24 * time.Hour

// postgresUniqueViolation is the error code Postgres returns when a UNIQUE
// constraint (here, users.email) is violated.
const postgresUniqueViolation = "23505"

type signupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// Signup handles POST /signup: it creates a new user and returns a JWT.
func Signup(pool *pgxpool.Pool, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req signupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		if req.Name == "" || req.Email == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "name, email and password are required")
			return
		}
		if len(req.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not process password")
			return
		}

		user, err := insertUser(r.Context(), pool, req.Name, req.Email, string(passwordHash))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation {
				writeError(w, http.StatusConflict, "email is already registered")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not create user")
			return
		}

		token, err := jwt.GenerateToken(user.ID, user.Email, jwtSecret, tokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not generate token")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(authResponse{Token: token, User: user})
	}
}

func insertUser(ctx context.Context, pool *pgxpool.Pool, name, email, passwordHash string) (models.User, error) {
	const query = `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, name, email, created_at, updated_at
	`

	var user models.User
	err := pool.QueryRow(ctx, query, name, email, passwordHash).Scan(
		&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt,
	)
	return user, err
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
