package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/justinas/nosurf"
	"github.com/skaldfm/skald/internal/auth"
	"github.com/skaldfm/skald/internal/backup"
	"github.com/skaldfm/skald/internal/config"
	"github.com/skaldfm/skald/internal/database"
	"github.com/skaldfm/skald/internal/handlers"
	"github.com/skaldfm/skald/internal/models"
	"github.com/skaldfm/skald/internal/views"
)

var version = "dev"

// noListFS wraps an http.FileSystem to disable directory listings: opening a
// directory returns "not exist" so http.FileServer responds 404 instead of
// rendering an enumerable index of uploaded files.
type noListFS struct {
	fs http.FileSystem
}

func (n noListFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if info.IsDir() {
		_ = f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}

// securityHeaders sets response headers that harden the app without breaking
// its inline scripts/styles (a Content-Security-Policy is intentionally omitted
// until the templates drop inline handlers). nosniff also stops browsers from
// content-sniffing uploaded files into an executable type. HSTS is only sent
// when the deployment is expected to be behind TLS.
func securityHeaders(tls bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "same-origin")
			if tls {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	_ = godotenv.Load() // optional .env file

	cfg := config.Load()

	// Open database
	db, err := database.Open(cfg.DBURL, cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Set up backup manager
	backupMgr := backup.NewManager(db, cfg.DataDir, cfg.DBURL, cfg.BackupRetain)

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
	sessionManager.Cookie.Secure = cfg.SecureCookies
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	// Periodically purge expired sessions from the store (scs does not do this
	// for custom stores). Runs until the process exits.
	if _, err := sessionStore.Cleanup(time.Hour); err != nil {
		log.Printf("Warning: session cleanup scheduler failed to start: %v", err)
	}

	// CSRF protection middleware
	csrfProtect := func(next http.Handler) http.Handler {
		h := nosurf.New(next)
		h.SetBaseCookie(http.Cookie{
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.SecureCookies,
			SameSite: http.SameSiteLaxMode,
		})
		// nosurf v1.2.0 defaults isTLS to true, which forces Referer checks
		// on all requests. Override to actually check for TLS.
		h.SetIsTLSFunc(func(r *http.Request) bool { return r.TLS != nil })
		h.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("CSRF failure: method=%s url=%s reason=%v", r.Method, r.URL, nosurf.Reason(r))
			http.Error(w, "CSRF token validation failed", http.StatusBadRequest)
		}))
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
	settingsStore := models.NewSiteSettingsStore(db)

	// Set global logo path resolver
	views.SetLogoPathFunc(func() string {
		s, err := settingsStore.Get()
		if err != nil {
			return ""
		}
		return s.LogoPath
	})

	// Set up router
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(sessionManager.LoadAndSave)
	r.Use(csrfProtect)
	r.Use(securityHeaders(cfg.SecureCookies))

	// Public routes (no auth required)
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// User uploads: everything under /uploads is private and served behind
	// RequireAuth (below), except /uploads/site/* which holds public branding
	// (the site logo, referenced on the unauthenticated login page). Directory
	// listing is disabled so the tree can't be enumerated.
	uploadsServer := http.StripPrefix("/uploads/", http.FileServer(noListFS{http.Dir(filepath.Join(cfg.DataDir, "uploads"))}))
	r.Handle("/uploads/site/*", uploadsServer)

	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/robots.txt")
	})
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Auth routes (public, but setup redirect handled by LoadUser)
	authHandler := handlers.NewAuthHandler(userStore, guestStore, sessionManager, cfg.OpenRegistration)
	r.Mount("/auth", authHandler.Routes())

	// Protected routes (auth required)
	r.Group(func(r chi.Router) {
		r.Use(auth.LoadUser(sessionManager, userStore))
		r.Use(auth.RequireAuth)

		// Private user uploads (episode/show/guest artwork, scripts, sponsor
		// order docs). Any authenticated user may fetch these; per-show scoping
		// is tracked separately (see docs/code-review-2026-07.md P1).
		r.Handle("/uploads/*", uploadsServer)

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
		assetHandler := handlers.NewAssetHandler(assetStore, episodeStore, cfg.DataDir)
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
			adminHandler := handlers.NewAdminHandler(backupMgr, db, userStore, guestStore, showStore, settingsStore, cfg.DataDir)
			r.Mount("/admin", adminHandler.Routes())
		})
	})

	// Start scheduled backups
	backupMgr.StartSchedule(cfg.BackupInterval)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No ReadTimeout/WriteTimeout: asset uploads and downloads can be large
		// and slow; ReadHeaderTimeout still bounds slowloris on the headers.
	}

	// Graceful shutdown on SIGINT/SIGTERM so in-flight requests finish and the
	// database is closed cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Skald %s starting on :%s", version, cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	}
}
