package config

import (
	"reflect"
	"testing"
)

func TestLoad_CORSAllowedOrigins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "x")

	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"single origin", "https://app.example.com", []string{"https://app.example.com"}},
		{"several origins, spaces and empties ignored", " https://a.com , https://b.com,, ", []string{"https://a.com", "https://b.com"}},
		{"unset means no origins", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CORS_ALLOWED_ORIGINS", tt.raw)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !reflect.DeepEqual(cfg.CORSAllowedOrigins, tt.want) {
				t.Errorf("CORSAllowedOrigins = %#v, want %#v", cfg.CORSAllowedOrigins, tt.want)
			}
		})
	}
}
