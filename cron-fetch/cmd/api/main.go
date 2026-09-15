// Command api is the CronFetch backend entrypoint.
package main

import (
	"log"
	"net/http"

	"github.com/kv13b/Crontfetch/internal/handlers"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health)

	addr := ":8080"
	log.Printf("CronFetch listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
