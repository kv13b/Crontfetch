// Package telegrambot listens for messages to the bot and links a user's
// Telegram chat to their account when they tap Start on a Connect link.
package telegrambot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/telegram"
)

const (
	// pollTimeoutSec is how long one getUpdates call waits for a message.
	pollTimeoutSec = 10
	// retryDelay is the pause after a failed poll, so an outage doesn't spin.
	retryDelay = 5 * time.Second
)

const (
	connectedText = "✅ Connected! New jobs matching your filters will be sent here."
	expiredText   = "This link has expired or was already used. Go back to CronFetch and tap Connect Telegram again."
	helpText      = "Hi! I send CronFetch job alerts. To link this chat, open CronFetch and tap Connect Telegram."
)

// Listen polls Telegram for messages to the bot until ctx is cancelled.
// shortcut: long polling works for one API instance only (Telegram rejects two pollers); switch to a webhook when deploying more than one.
func Listen(ctx context.Context, pool *pgxpool.Pool, botToken string) {
	slog.Info("telegrambot: listening")
	var offset int64
	for ctx.Err() == nil {
		updates, err := telegram.GetUpdates(ctx, botToken, offset, pollTimeoutSec)
		if err != nil {
			slog.Warn("telegrambot: polling", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message != nil {
				handleMessage(ctx, pool, botToken, *u.Message)
			}
		}
	}
}

func handleMessage(ctx context.Context, pool *pgxpool.Pool, botToken string, m telegram.Message) {
	chatID := strconv.FormatInt(m.Chat.ID, 10)

	reply := helpText
	if code, ok := startCode(m.Text); ok {
		userID, err := linkChat(ctx, pool, code, chatID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			reply = expiredText
		case err != nil:
			slog.Error("telegrambot: linking chat", "error", err)
			return // no reply; the user can tap the link again
		default:
			slog.Info("telegrambot: chat connected", "user_id", userID)
			reply = connectedText
		}
	}

	if err := telegram.SendMessage(ctx, botToken, chatID, reply); err != nil {
		slog.Warn("telegrambot: replying", "error", err)
	}
}

// startCode returns the code from "/start <code>", the message Telegram
// sends when someone taps Start on a t.me/<bot>?start=<code> link.
func startCode(text string) (string, bool) {
	fields := strings.Fields(text)
	if len(fields) != 2 || fields[0] != "/start" {
		return "", false
	}
	return fields[1], true
}

// linkChat uses up code and saves chatID on the code's owner, returning
// their user ID. It returns pgx.ErrNoRows if the code is unknown, expired or
// already used. Deleting and updating in one statement keeps a code single-use.
func linkChat(ctx context.Context, pool *pgxpool.Pool, code, chatID string) (string, error) {
	const query = `
		WITH used AS (
			DELETE FROM telegram_link_codes
			WHERE code = $1 AND expires_at > now()
			RETURNING user_id
		)
		UPDATE users SET telegram_chat_id = $2, updated_at = now()
		WHERE id = (SELECT user_id FROM used)
		RETURNING id
	`
	var userID string
	err := pool.QueryRow(ctx, query, code, chatID).Scan(&userID)
	return userID, err
}
