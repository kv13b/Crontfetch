// Command api is the CronFetch backend entrypoint.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/kv13b/Crontfetch/internal/config"
	"github.com/kv13b/Crontfetch/internal/db"
	"github.com/kv13b/Crontfetch/internal/handlers"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()
	log.Println("connected to database")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health(pool))

	addr := ":8080"
	log.Printf("CronFetch listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
