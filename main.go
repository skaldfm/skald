package main

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/nosurf"
	"github.com/mhermansson/skald/internal/auth"
	"github.com/mhermansson/skald/internal/backup"
	"github.com/mhermansson/skald/internal/config"
	"github.com/mhermansson/skald/internal/database"
	"github.com/mhermansson/skald/internal/handlers"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

var version = "dev"

func main() {
	cfg := config.Load()

	// Open database
	db, err := database.Open(cfg.DBURL, cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Set up backup manager
	backupMgr := backup.NewManager(db, cfg.DataDir, cfg.BackupRetain)

	// Pre-migration backup (skip if no migrations table yet = fresh DB)
	var hasMigrations int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&hasMigrations); err == nil && hasMigrations > 0 {
		if _, err := backupMgr.Create("pre-migration"); err != nil {
			log.Printf("Warning: pre-migration backup failed: %v", err)
		}
		_ = backupMgr.Prune()
	}

	// Run migrations
	if err := database.Migrate(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Load templates
	if err := views.Load("templates"); err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Session manager (sessions stored server-side in SQLite, survive restarts)
	sessionStore := auth.NewSQLiteStore(db)
	sessionManager := scs.New()
	sessionManager.Store = sessionStore
	sessionManager.Lifetime = 30 * 24 * time.Hour
	sessionManager.Cookie.Secure = false // allow HTTP for local dev
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	// CSRF protection middleware
	csrfProtect := func(next http.Handler) http.Handler {
		h := nosurf.New(next)
		h.SetBaseCookie(http.Cookie{
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
		})
		return h
	}

	// Stores
	showStore := models.NewShowStore(db)
	episodeStore := models.NewEpisodeStore(db)
	assetStore := models.NewAssetStore(db)
	guestStore := models.NewGuestStore(db)
	tagStore := models.NewTagStore(db)
	sponsorshipStore := models.NewSponsorshipStore(db)
	userStore := models.NewUserStore(db)

	// Set up router
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(sessionManager.LoadAndSave)
	r.Use(csrfProtect)

	// Public routes (no auth required)
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(filepath.Join(cfg.DataDir, "uploads")))))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Auth routes (public, but setup redirect handled by LoadUser)
	authHandler := handlers.NewAuthHandler(userStore, guestStore, sessionManager)
	r.Mount("/auth", authHandler.Routes())

	// Protected routes (auth required)
	r.Group(func(r chi.Router) {
		r.Use(auth.LoadUser(sessionManager, userStore))
		r.Use(auth.RequireAuth)

		// Home / Dashboard
		dashboardHandler := handlers.NewDashboardHandler(episodeStore, showStore, guestStore)
		r.Get("/", dashboardHandler.Dashboard)

		// Shows
		showHandler := handlers.NewShowHandler(showStore, episodeStore, guestStore, cfg.DataDir)
		r.Mount("/shows", showHandler.Routes())

		// Episodes
		episodeHandler := handlers.NewEpisodeHandler(episodeStore, showStore, assetStore, guestStore, tagStore, sponsorshipStore, cfg.DataDir)
		r.Mount("/episodes", episodeHandler.Routes())

		// Assets (upload/download/delete routes)
		assetHandler := handlers.NewAssetHandler(assetStore, cfg.DataDir)
		r.Post("/episodes/{episodeID}/assets", assetHandler.Upload)
		r.Mount("/assets", assetHandler.Routes())

		// Kanban board
		kanbanHandler := handlers.NewKanbanHandler(episodeStore, showStore)
		r.Mount("/kanban", kanbanHandler.Routes())

		// Calendar and Timeline
		calendarHandler := handlers.NewCalendarHandler(episodeStore, showStore)
		r.Mount("/calendar", calendarHandler.Routes())
		timelineHandler := handlers.NewTimelineHandler(episodeStore, showStore)
		r.Mount("/timeline", timelineHandler.Routes())

		// Guests
		guestHandler := handlers.NewGuestHandler(guestStore, cfg.DataDir)
		r.Mount("/guests", guestHandler.Routes())

		// Sponsorships
		sponsorshipHandler := handlers.NewSponsorshipHandler(sponsorshipStore, episodeStore, cfg.DataDir)
		r.Mount("/sponsorships", sponsorshipHandler.Routes())

		// Prompter
		prompterHandler := handlers.NewPrompterHandler(episodeStore)
		r.Get("/prompter/{id}", prompterHandler.Prompter)

		// Profile
		r.Get("/profile", authHandler.ProfileForm)
		r.Post("/profile", authHandler.ProfileUpdate)

		// Admin (requires admin role)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			adminHandler := handlers.NewAdminHandler(backupMgr, userStore)
			r.Mount("/admin", adminHandler.Routes())
		})
	})

	// Start scheduled backups
	backupMgr.StartSchedule(cfg.BackupInterval)

	log.Printf("Skald %s starting on :%s", version, cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
