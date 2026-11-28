package telegrambot

import "testing"

func TestStartCode(t *testing.T) {
	tests := []struct {
		text     string
		wantCode string
		wantOK   bool
	}{
		{"/start c0gjTjYj8yuksrBIIbYBPg", "c0gjTjYj8yuksrBIIbYBPg", true},
		{"  /start   abc  ", "abc", true},
		{"/start", "", false},
		{"hi", "", false},
		{"/start a b", "", false},
		{"/help abc", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		code, ok := startCode(tt.text)
		if code != tt.wantCode || ok != tt.wantOK {
			t.Errorf("startCode(%q) = %q, %v; want %q, %v", tt.text, code, ok, tt.wantCode, tt.wantOK)
		}
	}
}
