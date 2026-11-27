package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/middleware"
	"github.com/kv13b/Crontfetch/internal/telegram"
)

// linkCodeTTL is how long a "Connect Telegram" link works.
const linkCodeTTL = 10 * time.Minute

type telegramStatusResponse struct {
	Connected bool `json:"connected"`
}

type telegramLinkResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// newLinkCode returns an unguessable one-time code. 16 random bytes in
// URL-safe base64 is 22 characters, within Telegram's limit for a start
// parameter (64 characters of A-Z, a-z, 0-9, _ and -).
func newLinkCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// telegramLinkURL opens the bot in Telegram with code as the start
// parameter, so tapping Start sends the bot "/start <code>".
func telegramLinkURL(botUsername, code string) string {
	return "https://t.me/" + url.PathEscape(botUsername) + "?start=" + url.QueryEscape(code)
}

// botName looks up the bot's username once and reuses it. A failed lookup
// isn't cached, so the next request tries again.
type botName struct {
	mu       sync.Mutex
	username string
}

func (b *botName) get(ctx context.Context, botToken string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.username == "" {
		name, err := telegram.BotUsername(ctx, botToken)
		if err != nil {
			return "", err
		}
		b.username = name
	}
	return b.username, nil
}

// TelegramStatus handles GET /me/telegram: whether the caller has connected
// a Telegram chat for job alerts.
func TelegramStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		var connected bool
		err := pool.QueryRow(r.Context(),
			`SELECT telegram_chat_id IS NOT NULL FROM users WHERE id = $1`,
			claims.UserID,
		).Scan(&connected)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			slog.Error("telegram status: looking up", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not look up telegram status")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(telegramStatusResponse{Connected: connected})
	}
}

// TelegramLink handles POST /me/telegram/link: makes a fresh one-time code
// and returns the t.me link that carries it. The caller's older codes are
// dropped, so only the newest link works.
func TelegramLink(pool *pgxpool.Pool, botToken string) http.HandlerFunc {
	bot := &botName{}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}
		if botToken == "" {
			writeError(w, http.StatusServiceUnavailable, "job alerts are not configured on the server")
			return
		}

		username, err := bot.get(r.Context(), botToken)
		if err != nil {
			slog.Error("telegram link: looking up bot username", "error", err)
			writeError(w, http.StatusBadGateway, "could not reach Telegram, please try again")
			return
		}

		code, err := newLinkCode()
		if err != nil {
			slog.Error("telegram link: generating code", "error", err)
			writeError(w, http.StatusInternalServerError, "could not create link")
			return
		}

		// Clear this user's old codes (and anyone's expired ones) in the same
		// statement as the insert.
		const query = `
			WITH cleared AS (
				DELETE FROM telegram_link_codes WHERE user_id = $1 OR expires_at < now()
			)
			INSERT INTO telegram_link_codes (code, user_id, expires_at)
			VALUES ($2, $1, $3)
		`
		expiresAt := time.Now().Add(linkCodeTTL).UTC()
		if _, err := pool.Exec(r.Context(), query, claims.UserID, code, expiresAt); err != nil {
			slog.Error("telegram link: saving code", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not create link")
			return
		}

		slog.Info("telegram link: created", "user_id", claims.UserID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(telegramLinkResponse{
			URL:       telegramLinkURL(username, code),
			ExpiresAt: expiresAt,
		})
	}
}

// TelegramDisconnect handles DELETE /me/telegram: stops sending the
// caller's job alerts to Telegram.
func TelegramDisconnect(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization token")
			return
		}

		_, err := pool.Exec(r.Context(),
			`UPDATE users SET telegram_chat_id = NULL, updated_at = now() WHERE id = $1`,
			claims.UserID,
		)
		if err != nil {
			slog.Error("telegram disconnect: updating", "error", err, "user_id", claims.UserID)
			writeError(w, http.StatusInternalServerError, "could not disconnect telegram")
			return
		}

		slog.Info("telegram: disconnected", "user_id", claims.UserID)
		w.WriteHeader(http.StatusNoContent)
	}
}
