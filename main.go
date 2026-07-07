package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
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
	"github.com/skaldfm/skald/internal/logging"
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

// maxBodyBytes caps the size of request bodies (uploads spool to disk during
// multipart parsing, so an unbounded body is a disk-exhaustion vector). Applied
// to methods that carry a body; GETs are untouched.
func maxBodyBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
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

	startTime := time.Now()

	cfg := config.Load()

	logger := logging.Setup(cfg.LogLevel, cfg.LogFormat)

	// Only SQLite is wired up; a stray SKALD_DB_TYPE=postgres would otherwise
	// fall through and open a garbage SQLite path. Fail loudly instead.
	if cfg.DBType != "sqlite" {
		slog.Error("unsupported SKALD_DB_TYPE: only \"sqlite\" is implemented", "value", cfg.DBType)
		os.Exit(1)
	}

	// Open database
	db, err := database.Open(cfg.DBURL, cfg.DataDir)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// Set up backup manager
	backupMgr := backup.NewManager(db, cfg.DataDir, cfg.DBURL, cfg.BackupRetain)

	// Pre-migration backup (skip if no migrations table yet = fresh DB)
	var hasMigrations int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&hasMigrations); err == nil && hasMigrations > 0 {
		if _, err := backupMgr.Create("pre-migration"); err != nil {
			slog.Warn("pre-migration backup failed", "err", err)
		}
		_ = backupMgr.Prune()
	}

	// Run migrations
	if err := database.Migrate(db, assetFS("migrations")); err != nil {
		slog.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	// Load templates
	if err := views.Load(assetFS("templates")); err != nil {
		slog.Error("failed to load templates", "err", err)
		os.Exit(1)
	}

	// Session manager (sessions stored server-side in SQLite, survive restarts)
	sessionStore := auth.NewSQLiteStore(db)
	sessionManager := scs.New()
	sessionManager.Store = sessionStore
	sessionManager.Lifetime = 30 * 24 * time.Hour
	sessionManager.IdleTimeout = 14 * 24 * time.Hour // log out sessions idle for 2 weeks
	sessionManager.Cookie.Secure = cfg.SecureCookies
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	// Periodically purge expired sessions from the store (scs does not do this
	// for custom stores). Runs until the process exits.
	if _, err := sessionStore.Cleanup(time.Hour); err != nil {
		slog.Warn("session cleanup scheduler failed to start", "err", err)
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
			slog.Warn("CSRF failure", "method", r.Method, "url", r.URL.String(), "reason", nosurf.Reason(r))
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

	// Set global logo path resolver (cached; refreshed on logo update)
	views.SetLogoPathFunc(settingsStore.LogoPath)

	// Set up router
	r := chi.NewRouter()
	// RealIP trusts X-Forwarded-For/X-Real-IP, which is only safe behind a
	// reverse proxy that sets them — Skald's documented deployment. It gives the
	// real client IP for logging and login rate-limiting; a direct-to-internet
	// deployment should front the app with such a proxy.
	r.Use(middleware.RealIP) //nolint:staticcheck // trusted-proxy deployment; see comment above
	r.Use(logging.RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(maxBodyBytes(cfg.MaxUploadBytes))
	r.Use(sessionManager.LoadAndSave)
	r.Use(csrfProtect)
	r.Use(securityHeaders(cfg.SecureCookies))

	// Public routes (no auth required)
	staticFS := assetFS("static")
	fileServer := http.FileServer(http.FS(staticFS))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// User uploads: everything under /uploads is private and served behind
	// RequireAuth (below), except /uploads/site/* which holds public branding
	// (the site logo, referenced on the unauthenticated login page). Directory
	// listing is disabled so the tree can't be enumerated.
	uploadsServer := http.StripPrefix("/uploads/", http.FileServer(noListFS{http.Dir(filepath.Join(cfg.DataDir, "uploads"))}))
	r.Handle("/uploads/site/*", uploadsServer)

	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, staticFS, "robots.txt")
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

	// Prometheus metrics (text exposition, no client library). Unauthenticated
	// like /health — firewall the port or scrape it over a private network. The
	// last-backup gauge is the one alert a self-hoster needs; it reports 0 when
	// no backup exists yet so "backup age" alerts fire.
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		var lastBackup int64
		if ts, ok := backupMgr.LastBackupTime(); ok {
			lastBackup = ts.Unix()
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprint(w,
			"# HELP skald_up Always 1 when the process is scraped.\n"+
				"# TYPE skald_up gauge\nskald_up 1\n"+
				"# HELP skald_uptime_seconds Seconds since process start.\n"+
				"# TYPE skald_uptime_seconds gauge\n")
		fmt.Fprintf(w, "skald_uptime_seconds %d\n", int64(time.Since(startTime).Seconds()))
		fmt.Fprint(w, "# HELP skald_http_requests_total Total HTTP requests handled.\n"+
			"# TYPE skald_http_requests_total counter\n")
		fmt.Fprintf(w, "skald_http_requests_total %d\n", logging.HTTPRequestCount())
		fmt.Fprint(w, "# HELP skald_goroutines Current number of goroutines.\n"+
			"# TYPE skald_goroutines gauge\n")
		fmt.Fprintf(w, "skald_goroutines %d\n", runtime.NumGoroutine())
		fmt.Fprint(w, "# HELP skald_memory_alloc_bytes Currently allocated heap bytes.\n"+
			"# TYPE skald_memory_alloc_bytes gauge\n")
		fmt.Fprintf(w, "skald_memory_alloc_bytes %d\n", mem.Alloc)
		fmt.Fprint(w, "# HELP skald_last_backup_timestamp_seconds Unix time of the most recent backup (0 if none).\n"+
			"# TYPE skald_last_backup_timestamp_seconds gauge\n")
		fmt.Fprintf(w, "skald_last_backup_timestamp_seconds %d\n", lastBackup)
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
		// Route the net/http server's own error logging through slog too.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
		// No ReadTimeout/WriteTimeout: asset uploads and downloads can be large
		// and slow; ReadHeaderTimeout still bounds slowloris on the headers.
	}

	// Graceful shutdown on SIGINT/SIGTERM so in-flight requests finish and the
	// database is closed cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting", "version", version, "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("graceful shutdown failed", "err", err)
	}
}
