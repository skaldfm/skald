# Changelog

All notable changes to Skald will be documented in this file.

## [Unreleased]

### Added
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
- Home dashboard page
- **Shows** — full CRUD (list, create, view, edit, delete)
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
  - Auto-scrolling with adjustable speed (0.5-10 px/frame)
  - Play/pause, font size control (16-72px)
  - Fullscreen mode, mirror mode (for reflected setups)
  - Center-line reading guide
  - Auto-hiding controls during playback
  - Keyboard shortcuts (space, arrows, f, m)
  - Touch-friendly for tablet use
- **Docker** — multi-stage Dockerfile and docker-compose.yml
