// Command api is the CronFetch backend entrypoint.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/kv13b/Crontfetch/internal/config"
	"github.com/kv13b/Crontfetch/internal/db"
	"github.com/kv13b/Crontfetch/internal/handlers"
	"github.com/kv13b/Crontfetch/internal/middleware"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connecting to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("connected to database")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health(pool))
	mux.HandleFunc("POST /signup", handlers.Signup(pool, cfg.JWTSecret))
	mux.HandleFunc("POST /login", handlers.Login(pool, cfg.JWTSecret))

	addr := ":8080"
	slog.Info("CronFetch listening", "addr", addr)
	if err := http.ListenAndServe(addr, middleware.Logging(mux)); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
