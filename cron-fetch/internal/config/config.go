// Package config loads CronFetch's runtime configuration from the environment.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds everything CronFetch reads from the environment at startup.
type Config struct {
	DatabaseURL      string
	TelegramBotToken string
	TelegramChatID   string
	JWTSecret        string
}

// Load reads .env (if present) into the process environment, then builds a Config.
func Load() (Config, error) {
	// Ignore the error: in production there may be no .env file, and real
	// env vars set by the host/container should be used instead.
	_ = godotenv.Load()

	cfg := Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is not set")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is not set")
	}

	return cfg, nil
}
