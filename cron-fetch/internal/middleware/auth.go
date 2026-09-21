package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kv13b/Crontfetch/internal/jwt"
)

// contextKey is unexported so no other package can accidentally collide
// with this key when storing something else in the request context.
type contextKey int

const claimsContextKey contextKey = iota

// Auth returns middleware that requires a valid JWT in the Authorization
// header, in the form "Bearer <token>". Requests without one, or with an
// invalid/expired one, are rejected with 401 before reaching the handler.
// On success, the token's claims are attached to the request context.
func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || tokenString == "" {
				writeUnauthorized(w)
				return
			}

			claims, err := jwt.ValidateToken(tokenString, secret)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext retrieves the authenticated caller's claims. It only
// returns something once a request has passed through Auth.
func ClaimsFromContext(ctx context.Context) (*jwt.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*jwt.Claims)
	return claims, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "missing or invalid authorization token"})
}
