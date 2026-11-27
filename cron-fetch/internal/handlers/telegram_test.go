package handlers

import (
	"regexp"
	"testing"
)

// Telegram only accepts these characters in a start parameter, up to 64.
var startParamRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func TestNewLinkCode(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		code, err := newLinkCode()
		if err != nil {
			t.Fatalf("newLinkCode() error = %v", err)
		}
		if !startParamRe.MatchString(code) {
			t.Fatalf("code %q isn't a valid Telegram start parameter", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}
}

func TestTelegramLinkURL(t *testing.T) {
	got := telegramLinkURL("CronFetchBot", "abc_123-XYZ")
	if want := "https://t.me/CronFetchBot?start=abc_123-XYZ"; got != want {
		t.Errorf("telegramLinkURL() = %q, want %q", got, want)
	}
}
