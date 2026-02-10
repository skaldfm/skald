# Skald

A self-hosted podcast management tool for indie podcasters who want control over their workflow and data.

## What Is This?

Most podcast tools are cloud-only, subscription-heavy, and ignore the production workflow. Skald covers the full lifecycle — from episode planning and scripting to a built-in prompter for recording — all running on your own infrastructure.

Single Go binary, SQLite database, no external dependencies. Deploy with Docker or run directly.

## Features

### Shows & Episodes
- **Multi-show support** — manage multiple podcasts in one instance
- **Full episode metadata** — title, season/episode numbers (S01E01 format with uniqueness enforcement), description, publish date, status, script, show notes
- **Episode artwork** — per-episode cover art upload with thumbnails in list views
- **Show artwork** — cover art for each podcast
- **Tags** — flexible tagging system for episodes

### Production Pipeline
- **Six-stage workflow** — Idea → Research → Scripted → Recorded → Edited → Published
- **Kanban board** — drag-and-drop episodes between pipeline stages
- **Status tracking** — color-coded badges and pipeline progress bars

### Views
- **List view** — sortable, filterable episode table with search, show filter, and status filter
- **Kanban board** — visual drag-and-drop board grouped by pipeline stage
- **Calendar** — monthly grid showing episodes on their publish dates, with navigation and show filtering
- **Timeline** — horizontal scrollable timeline with month and week zoom levels, auto-scrolls to current period

### Dashboard
- Overview stats (shows, episodes, published count, guests)
- Production pipeline bar showing episode distribution across stages
- Recently updated episodes and upcoming schedule
- Per-show cards with artwork and mini pipeline bars

### Script Prompter
Built-in teleprompter for recording sessions:
- Adjustable scroll speed (1–20, quadratic curve for fine control at low speeds)
- Font size control (16–72px)
- Fullscreen and mirror mode (for reflected prompter setups)
- Markdown rendering — **bold** and _underline_ in scripts
- Font color presets (white, yellow, green, cyan)
- Background color presets (black, dark blue, dark green, dark gray)
- Center text toggle
- Controls at top of screen for tablet ergonomics
- All preferences saved to localStorage
- Keyboard shortcuts (Space, arrows, +/-, F, M)
- Tablet-friendly — works well in portrait and landscape

### People (Hosts & Guests)
- Profiles with name, email, bio, website, company, podcast
- Photo upload with avatar display
- Social links — Twitter/X, Instagram, LinkedIn, Mastodon
- **Host flag** — mark people as hosts; host pickers only show flagged people
- **Show hosts** — define default hosts per show, auto-inherited by new episodes
- **Per-episode override** — change hosts on individual episodes without affecting the show default
- Link guests to episodes with roles
- People list shows avatar thumbnails and which shows each person has appeared on
- Searchable tag-picker for linking guests and hosts to episodes

### Sponsorships
- Sponsor deal tracking with ad copy, CPM, total cost, average listens
- Drop date and payment due date
- Order document upload
- Link sponsors to episodes via searchable tag-picker
- Financial overview on sponsor detail pages

### Assets
- File attachments per episode (scripts, audio, artwork, notes)
- Upload, download, and delete

### Backups
- **Automatic pre-migration backups** — safety net before any schema changes
- **Scheduled backups** — configurable interval (default daily), automatic retention/pruning (default 14)
- **Manual backups** — create and download from the admin page
- Uses SQLite `VACUUM INTO` for consistent snapshots safe with WAL mode

## Tech Stack

- **Backend:** Go with [chi](https://github.com/go-chi/chi) router
- **Frontend:** Go `html/template` + [HTMX](https://htmx.org/) + vanilla JS (tag-picker component)
- **Markdown:** [Goldmark](https://github.com/yuin/goldmark) for script/show notes rendering
- **CSS:** [Tailwind CSS](https://tailwindcss.com/) v4 with full dark mode support
- **Database:** SQLite (WAL mode) via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **Deployment:** Single Docker container

## Quick Start

### Docker (recommended)

```sh
docker compose up -d
```

Skald will be available at `http://localhost:7707`. Data is persisted in a Docker volume.

### From Source

```sh
go build -o skald .
./skald
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SKALD_PORT` | `7707` | Server port |
| `SKALD_DATA_DIR` | `./data` | Data directory (database, uploads, backups) |
| `SKALD_DB_TYPE` | `sqlite` | Database type |
| `SKALD_DB_URL` | `{DataDir}/skald.db` | Database connection string |
| `SKALD_BACKUP_INTERVAL` | `24h` | Scheduled backup frequency (Go duration) |
| `SKALD_BACKUP_RETAIN` | `14` | Number of backups to keep |

## Screenshots

*Coming soon*

## License

AGPL-3.0 — see [LICENSE](LICENSE).
