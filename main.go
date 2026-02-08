package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mhermansson/podforge/internal/config"
	"github.com/mhermansson/podforge/internal/database"
	"github.com/mhermansson/podforge/internal/handlers"
	"github.com/mhermansson/podforge/internal/models"
	"github.com/mhermansson/podforge/internal/views"
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

	// Load templates
	if err := views.Load("templates"); err != nil {
		log.Fatalf("Failed to load templates: %v", err)
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

	// Home
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		if err := views.Render(w, "home.html", nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Stores
	showStore := models.NewShowStore(db)
	episodeStore := models.NewEpisodeStore(db)
	assetStore := models.NewAssetStore(db)
	guestStore := models.NewGuestStore(db)
	tagStore := models.NewTagStore(db)

	// Shows
	showHandler := handlers.NewShowHandler(showStore)
	r.Mount("/shows", showHandler.Routes())

	// Episodes
	episodeHandler := handlers.NewEpisodeHandler(episodeStore, showStore, assetStore, guestStore, tagStore)
	r.Mount("/episodes", episodeHandler.Routes())

	// Assets (upload/download/delete routes)
	assetHandler := handlers.NewAssetHandler(assetStore, cfg.DataDir)
	r.Mount("/", assetHandler.Routes())

	// Kanban board
	kanbanHandler := handlers.NewKanbanHandler(episodeStore, showStore)
	r.Mount("/kanban", kanbanHandler.Routes())

	// Guests
	guestHandler := handlers.NewGuestHandler(guestStore)
	r.Mount("/guests", guestHandler.Routes())

	// Prompter
	prompterHandler := handlers.NewPrompterHandler(episodeStore)
	r.Get("/prompter/{id}", prompterHandler.Prompter)

	log.Printf("PodForge starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
