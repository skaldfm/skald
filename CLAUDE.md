# Skald

Self-hosted podcast management tool for indie podcasters who want control over their workflow and data. Covers the full production lifecycle: planning, scripting, recording support (prompter), and publishing coordination.

## Tech Stack

- **Backend:** Go, using chi or echo for routing
- **Frontend:** HTMX + Go `html/template` — no JS framework
- **Database:** SQLite default (single file), PostgreSQL support later
- **CSS:** Tailwind CSS
- **Deployment:** Single Docker container, docker-compose

### Rationale

Single binary + SQLite = trivial to self-host. HTMX gives dynamic UX without JS complexity. Can add PostgreSQL and multi-user later without rewriting.

## Architecture

```
Go backend (chi/echo)
  ├── HTTP handlers → html/template rendering
  ├── Business logic layer
  └── SQLite / PostgreSQL via database/sql
```

## MVP Scope

### In v1

- **Shows** — multiple podcasts per instance
- **Episodes** — title, number, season, description, publish date, status, script (markdown), show notes
- **Production pipeline** — Idea → Research → Scripted → Recorded → Edited → Published
- **Guests** — name, role, contact, linked to episodes
- **Assets** — file attachments per episode (scripts, audio, artwork)
- **Tags** — flexible tagging
- **Views** — Kanban (drag-and-drop by status), Timeline/calendar (color-coded by status), List (sortable/filterable)
- **Script prompter** — scrolling teleprompter with adjustable speed, play/pause, segment markers, font controls, fullscreen. Must work well on tablet.

### Explicitly not in v1

RSS feed generation, analytics, Obsidian/Notion integration, multi-user/teams, audio processing, social media scheduling, AI features, API.

## Data Model

```sql
-- Core tables
Show       (id, name, description, artwork, website, podcast_host, color,
            created_at, updated_at)
Episode    (id, show_id FK, title, episode_number, season_number, description,
            status ENUM, publish_date, script TEXT, show_notes TEXT, artwork,
            created_at, updated_at)
Guest      (id, name, email, bio, website, image, company, podcast,
            twitter, instagram, linkedin, mastodon, is_host,
            created_at, updated_at)
EpisodeGuest (episode_id, guest_id, role)
ShowHost   (show_id, guest_id)
Sponsorship (id, name, contact_name, contact_email, website, ad_copy,
            cpm, total_cost, avg_listens, drop_date, payment_due,
            order_doc, notes, created_at, updated_at)
EpisodeSponsor (episode_id, sponsorship_id)
Asset      (id, episode_id FK, filename, filepath, filetype, filesize,
            asset_type ENUM, created_at)
Tag        (id, name)
EpisodeTag (episode_id, tag_id)
User       (id, username, email, password_hash, role, created_at, updated_at)
UserShow   (user_id, show_id)
SiteSetting (key, value)
```

Status enum: `idea`, `research`, `scripted`, `recorded`, `edited`, `published`
Asset type enum: `script`, `audio`, `artwork`, `notes`, `other`

## Project Structure

```
skald/
├── CLAUDE.md
├── README.md
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── main.go
├── internal/
│   ├── auth/          # Authentication, sessions, RBAC
│   ├── backup/        # Backup/restore manager
│   ├── config/
│   ├── database/      # Connection, migrations
│   ├── handlers/      # HTTP handlers (thin)
│   ├── models/        # Data models and queries
│   └── views/         # Template rendering helpers
├── migrations/        # Numbered SQL files
├── templates/
│   ├── layouts/
│   ├── episodes/
│   ├── shows/
│   ├── guests/
│   ├── sponsorships/
│   ├── prompter/
│   ├── admin/
│   ├── auth/
│   └── components/    # Reusable partials
├── static/
│   ├── css/
│   ├── js/            # HTMX + minimal JS
│   └── images/
└── data/              # SQLite DB location (gitignored)
```

## Go Guidelines (Project-Specific)

- Standard library first, third-party only when justified
- `database/sql` with raw SQL or sqlc — no ORM
- Thin handlers, business logic in models/services
- Wrap errors with context
- Tests: integration for handlers, unit for logic

## Frontend Guidelines

- HTMX for all dynamic interactions (status changes, drag-and-drop, search)
- Progressive enhancement — works without JS, HTMX makes it better
- Responsive — must work on tablet (prompter use case)
- Minimal UI, no clutter

## Database Guidelines

- All schema changes via numbered migration files in `migrations/`
- Foreign keys enabled
- Transactions for multi-step operations

## Docker

- Multi-stage build: `golang` image → `alpine` runtime
- Single container, one port
- Volume mount for `data/` (SQLite + uploads)

## Configuration

```
SKALD_PORT=7707
SKALD_DATA_DIR=./data
SKALD_DB_TYPE=sqlite
SKALD_DB_URL=              # PostgreSQL connection string if needed
SKALD_SECRET_KEY=          # Session signing
```

## Prior Art

- Airtable Podcast Editorial Calendar (workflow inspiration)
- Castopod (self-hosted RSS hosting — no workflow)
- Uptime Kuma (excellent self-hosted tool UX reference)
- Planka (self-hosted Kanban UI reference)
