// Package telegram sends messages through the Telegram Bot API.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// apiBase is a variable so tests can point it at a local server.
var apiBase = "https://api.telegram.org"

var httpClient = &http.Client{Timeout: 15 * time.Second}

// SendMessage sends text to chatID via the bot identified by botToken.
func SendMessage(ctx context.Context, botToken, chatID, text string) error {
	body, err := json.Marshal(map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return fmt.Errorf("encoding telegram message: %w", err)
	}

	endpoint := apiBase + "/bot" + url.PathEscape(botToken) + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending telegram message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding telegram response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram API error: %s", result.Description)
	}
	return nil
}
