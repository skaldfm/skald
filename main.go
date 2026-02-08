package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mhermansson/podforge/internal/config"
	"github.com/mhermansson/podforge/internal/database"
)

func main() {
	cfg := config.Load()

	// Open database
	db, err := database.Open(cfg.DBURL, cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.Migrate(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Set up router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Serve static files
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Placeholder root
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<h1>PodForge</h1><p>Coming soon.</p>"))
	})

	log.Printf("PodForge starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
