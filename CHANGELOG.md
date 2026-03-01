# Changelog

All notable changes to Skald will be documented in this file.

## [Unreleased]

## [0.2.0] - 2026-03-01

### Added
- **Backup restore** — restore from any backup directly in the admin UI; validates backup integrity, creates a safety "pre-restore" backup, atomically replaces the database, and restarts the application
- **Episode number hints** — new episode form auto-suggests the next available episode number when selecting a show or season, with a hint showing which numbers are taken
- **Show-scoped guests & sponsors** — non-admin users only see guests and sponsors linked to episodes in their assigned shows; episode edit pickers remain global for discoverability
- **Admin settings page** — new Settings tab in admin panel at `/admin/settings`
- **Custom logo upload** — admins can upload a site logo that replaces the default in nav bar and login page; remove button to revert to default
- **robots.txt** — disallows all crawling for public-facing instances
- **PUID/PGID support** — Docker container supports custom user/group IDs via `PUID`/`PGID` environment variables (defaults to 1000:1000)
- **Role-based access control** — three roles: admin (full access), editor (edit assigned shows), viewer (read-only assigned shows); show-scoped permissions via `user_shows` join table
- **Show assignment admin** — admin page at `/admin/users/{id}/shows` to assign shows to editors/viewers via tag-picker
- **Authentication** — multi-user auth with bcrypt passwords and server-side sessions (SQLite-backed), first-run setup wizard, login/logout
- **User roles** — first user gets `admin` role; admin-only route gating with `RequireAdmin` middleware
- **Profile settings** — logged-in users can update name, email, and password at `/profile`
- **User management** — admin page at `/admin/users` to list users, create users, set role (admin/editor/viewer), and delete accounts (with self-protection guards)
- **Open registration** — optional self-service account creation via `SKALD_OPEN_REGISTRATION=true`, disabled by default; new registrations get viewer role
- **Admin sub-navigation** — tabbed nav across admin pages (Users, Backups, Settings), Users as default landing tab
- **Show hosts** — define default hosts per show, auto-inherited by new episodes, with per-episode override
- **Sponsorships** — sponsor deal tracking with ad copy, CPM, total cost, average listens, order document upload, episode linking via tag-picker
- **Guest enhancements** — photo upload, social links (Twitter/X, Instagram, LinkedIn, Mastodon), company/podcast fields
- **Episode artwork** — per-episode cover art upload with thumbnails in list views
- **Calendar view** — monthly grid showing episodes on their publish dates, with navigation and show filtering
- **Timeline view** — horizontal scrollable timeline with month and week zoom levels
- **Dashboard** — overview stats, production pipeline bar, recent episodes, upcoming schedule, per-show cards
- **Backups** — automatic pre-migration backups, scheduled backups with configurable interval/retention, manual backup/download from admin page
- **Searchable tag-picker** — reusable component for linking guests, hosts, and sponsors to episodes/shows
- **People section** — renamed "Guests" to "People" throughout the UI to better reflect that hosts live there too
- **Host flag** — `is_host` boolean on people; host pickers (show edit, episode edit) now only show people flagged as hosts
- **`.env` file support** — optional `.env` file via godotenv for configuration

### Changed
- Prompter: quadratic speed curve for finer control at low speeds, font color/background presets, center text toggle, top-positioned controls for tablet ergonomics, localStorage persistence for all preferences
- **UI modernization** — status badges now have proper dark-mode colors, auth pages use consistent indigo palette, active nav link highlighting, mobile nav wraps instead of overflowing, self-hosted HTMX (no CDN), inline styles replaced with Tailwind classes, loading indicators on status changes and form submits, kanban drop animation, tag-picker keyboard navigation and ARIA accessibility, input borders fixed on auth/profile forms

### Fixed
- Kanban drag-and-drop was silently failing — missing CSRF token in the fetch POST
- Episode number uniqueness enforcement (per show+season)

## [0.1.0] - 2026-02-08

Initial working version.

### Infrastructure
- Project scaffolding: Go module, directory structure, Makefile
- Configuration loading from environment variables
- SQLite database with WAL mode, foreign keys, and busy timeout
- Migration runner with version tracking
- Initial database schema: shows, episodes, guests, assets, tags with linking tables
- Chi router with logging and recovery middleware
- Static file serving and health check endpoint (`/health`)
- Cross-compilation targets for linux/darwin/windows (amd64 + arm64)
- Template system with layout/component/page composition
- Base HTML layout with Tailwind CSS and HTMX
- Responsive navigation bar (desktop + mobile)
- Dark mode with toggle, localStorage persistence, system preference detection
- Docker multi-stage build and docker-compose.yml
- GitHub Actions CI (lint + build)

### Features
- **Shows** — full CRUD (list, create, view, edit, delete) with artwork upload
- **Episodes** — full CRUD with status pipeline
  - Filterable list view by show, status, and search text
  - Status pipeline UI with clickable status buttons
  - HTMX-powered inline status updates
  - Script editor (monospace) and show notes fields
  - Season/episode numbering (S1E3 format)
- **Kanban board** — drag-and-drop episodes between pipeline stages
  - Visual column layout for each status
  - HTML5 drag-and-drop with async status updates
  - Filterable by show
- **Guests** — full CRUD with episode linking
  - Contact details (email, bio, website)
  - Linked episodes with roles displayed on guest detail page
- **Assets** — file upload/download/delete per episode
  - Multipart upload (32MB limit)
  - Asset type categorization (script, audio, artwork, notes, other)
  - File size display with human-readable formatting
- **Tags** — comma-separated tagging on episodes
  - Auto-created on first use
  - Displayed as badges on episode detail page
- **Script prompter** (teleprompter)
  - Auto-scrolling with adjustable speed (0.5–10 px/frame)
  - Play/pause, font size control (16–72px)
  - Fullscreen mode, mirror mode (for reflected setups)
  - Center-line reading guide
  - Auto-hiding controls during playback
  - Keyboard shortcuts (space, arrows, f, m)
  - Touch-friendly for tablet use
