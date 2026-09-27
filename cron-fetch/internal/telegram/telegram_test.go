package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSendMessage(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	old := apiBase
	apiBase = srv.URL
	defer func() { apiBase = old }()

	if err := SendMessage(context.Background(), "TESTTOKEN", "12345", "hello"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if gotPath != "/botTESTTOKEN/sendMessage" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["chat_id"] != "12345" || gotBody["text"] != "hello" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestSendMessage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "chat not found"})
	}))
	defer srv.Close()

	old := apiBase
	apiBase = srv.URL
	defer func() { apiBase = old }()

	err := SendMessage(context.Background(), "TESTTOKEN", "bad-chat", "hello")
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("error = %v, want it to mention 'chat not found'", err)
	}
}

// TestSendMessage_Live sends a real message to your Telegram, using the same
// TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID as the app. It's skipped if they
// aren't set in the environment running the test.
func TestSendMessage_Live(t *testing.T) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		t.Skip("TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID not set")
	}

	err := SendMessage(context.Background(), token, chatID, "CronFetch: test message from telegram_test.go — safe to ignore.")
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
}
