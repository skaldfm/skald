# Skald — Code Review & Backlog (2026-07-07)

Findings from a deep multi-area review (security/auth, backend correctness, frontend/templates, infra/ops). Each item is verified against actual code. Severity and file:line noted so items can be picked up independently later.

Checkboxes are for tracking. **P0 is done (2026-07-07)** — built, vetted, and smoke-tested; the rest is outstanding.

---

## P0 — Fix first (security / data safety) — ✅ DONE

- [x] **`/uploads/*` served with no authentication** — `main.go:116` → fixed in `main.go`
  Was mounted in the public section with directory listing enabled. **Fix applied:** `/uploads/*` moved inside the `RequireAuth` group; `/uploads/site/*` kept public for branding (login-page logo lives there); added a `noListFS` wrapper so directory listing returns 404. Smoke-tested: `site/logo.png`→200, `5/secret.txt`→302-to-auth, `site/`→404.
  **Still open (P1):** any *authenticated* user can fetch any upload — per-show scoping of uploads is not yet enforced (entangled with the guest/sponsor scoping decision, P1 below).

- [x] **SQLite pragmas apply to only one pooled connection** — `database/database.go` → fixed
  **Fix applied:** pragmas moved into the DSN (`_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)`) so they apply to every connection; added `db.SetMaxOpenConns(1)` to serialize writes and avoid `SQLITE_BUSY`.

- [x] **Asset download has no access check (IDOR)** — `handlers/assets.go:103` → fixed
  **Fix applied:** `Download` now loads the episode and calls `requireShowAccess(ep.ShowID)`, mirroring `Delete`.

- [x] **XSS + silent data loss via `toJSON`** — `views.go:185` → fixed
  **Fix applied:** `toJSON` now returns `string` so html/template contextually escapes it (`'`→`&#39;`), which stops both the attribute break-out (O'Brien data loss) and attribute-injection XSS; browser decodes entities before `JSON.parse`, so parsing still works. Verified with an html/template render test. Also wrapped each `initPicker` call in try/catch (`tag-picker.js`) so one malformed item can't stop the other pickers from rendering their hidden inputs.

---

## P1 — Security (remaining)

> **Status (2026-07-07):** ✅ **P1 fully complete** — all security, lifecycle, and correctness items done and build/vet/test/smoke-verified, including login rate-limiting, password bounds, the episode-number NULL-season fix (migration 015), and a configurable upload cap. The only thing not addressed is a CSP (needs a template refactor to drop inline scripts — tracked under P2/frontend). Dependency advisories are also cleared (`govulncheck` clean). Next up: P2.

- [x] **Upload file type never validated** — fixed
  **Applied:** added `internal/handlers/upload.go` with image/doc extension allowlists (SVG excluded — scriptable); applied to episode/show artwork, guest image, sponsor order file; admin logo now uses the shared helper (drops `.svg`). Combined with `X-Content-Type-Options: nosniff` (added globally) this closes the stored-XSS-via-upload vector.

- [x] **Episode reassignment escapes scope** — `handlers/episodes.go` → fixed
  **Applied:** `Update` now checks `CanAccessShow` on the target show before accepting a `show_id` change.

- [x] **Guest & sponsorship detail/edit ignore show scoping** — fixed (decision: **show-scoped isolation**)
  **Applied:** added `GuestStore.AccessibleToShows` and `SponsorshipStore.AccessibleToShows` (accessible = linked to an episode in one of the user's shows, or — for guests — a host of one; **orphans** with no links stay visible so create-then-view works). Enforced centrally in `getGuest`/`getSponsorship`, returning **404** (not 403) so out-of-scope entities can't be enumerated. Admins bypass. Covered by unit tests (`internal/models/scoping_test.go`).

- [x] **Session & CSRF cookies hardcoded `Secure=false`** — fixed
  **Applied:** driven by `SKALD_SECURE_COOKIES` (default **true**); set `false` for plain-HTTP LAN access without a TLS proxy. Verified the CSRF cookie now carries `Secure`.

- [x] **Login user-enumeration (timing)** — `auth.go` → fixed
  **Applied:** `CheckDummyPassword` runs a bcrypt compare against a fixed hash when the account doesn't exist, equalizing response time. (The explicit "email already exists" register message is a minor secondary leak, left as-is.)

### Hardening (not bugs)
- [x] Security headers middleware — `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, HSTS when secure. **CSP intentionally omitted** (templates use inline `<script>`/`onclick`; needs a template refactor first).
- [x] Idle session timeout — `sessionManager.IdleTimeout = 14d` added (absolute lifetime stays 30d).
- [x] Password max length + login rate-limit/lockout — fixed
  **Applied:** `passwordProblem` helper enforces 8–72 chars (bcrypt truncates past 72) at register/setup/profile/admin-create; `loginLimiter` (in-memory, per-IP, 10 failures / 15 min → 429) throttles online guessing. Both unit-tested.

---

## P1 — Correctness & data safety

- [x] **`SKALD_BACKUP_INTERVAL=0` panics the process** — `backup.go` → fixed
  **Applied:** `StartSchedule` returns early (logs "disabled") on a non-positive interval. Verified: boots cleanly with `SKALD_BACKUP_INTERVAL=0`.

- [x] **Episode save is non-transactional, all errors swallowed** — fixed
  **Applied:** added atomic replace-all `SetEpisodeGuests`/`SetEpisodeHosts`/`SetEpisodeSponsorships` (each in its own transaction, matching the existing `SetEpisodeTags`/`SetShowHosts` pattern), and the `Update` handler now **propagates** their errors as 500s instead of `_ =` swallowing them. Guest/host role separation is unit-tested. Note: this is **per-relation** atomicity, not one global transaction across all link tables — a deliberate choice for simplicity given the app's scale; `SetMaxOpenConns(1)` already removed the `SQLITE_BUSY` trigger that caused the original silent drops.

- [x] **Multipart size limits are dead code** — fixed
  **Applied:** `maxBodyBytes` middleware wraps request bodies with `http.MaxBytesReader` on POST/PUT/PATCH, sized by `SKALD_MAX_UPLOAD_MB` (default 512 MB — generous for audio, but no longer unbounded).

- [x] **Expired sessions never cleaned up** — `main.go` → fixed
  **Applied:** `sessionStore.Cleanup(time.Hour)` started at boot.

- [x] **No graceful shutdown or server timeouts** — `main.go` → fixed
  **Applied:** `http.Server` with `ReadHeaderTimeout`/`IdleTimeout`, `signal.NotifyContext(SIGINT/SIGTERM)` + `srv.Shutdown`. Verified clean exit on SIGTERM. `/health` now does `db.PingContext` (returns 503 on a dead DB).

- [x] **Internal error text leaked to browsers** — fixed
  **Applied:** added `serverError(w, r, err)` helper (`internal/handlers/errors.go`) that logs the detail and returns a generic 500; replaced all 113 `http.Error(w, err.Error(), 500)` sites across the handlers, plus the admin restore message. Invalid episode status already returns 400 (validated against `models.Statuses`).

### Lower-severity correctness
- [x] Episode-number uniqueness NULL-season loophole — migration `015` replaces the index with an expression index on `COALESCE(season_number, -1)` so NULL-season duplicates are now rejected at the DB level (unit-tested). The app-level check-then-write race remains theoretically possible but is bounded by `SetMaxOpenConns(1)` and now backstopped by the DB constraint.
- [x] `migrate.go` treated any query error as "not applied" — now checks `errors.Is(err, sql.ErrNoRows)` and returns real errors.
- [x] Ignored `Atoi` errors — episode/season number parse errors now return 400 instead of silently storing `0`.
- [x] `assets.go` stored absolute `Filepath` — now stores a data-dir-relative path (with a `resolvePath` helper that still handles legacy absolute rows).
- [x] `views.Render` wrote directly to the ResponseWriter — now renders into a buffer first, so a template error leaves the response clean.
- [x] Dashboard counted unscoped guests — now scoped to the user's shows (admins get all).

---

## P2 — Infra / ops

> **Status (2026-07-07):** ✅ All six main infra bugs fixed in commit `8944d29`, build/test/lint/standalone-run verified. Uploads-in-backup is deliberately **not** auto-tarred — see note under that item. Remaining: lower-severity ops cleanups (secret key, db_type guard, down migrations, README) and the two ops improvements (slog, /metrics).

- [x] **Release binaries are unrunnable standalone** — `release.yml`, `main.go:50,55,114` — ✅ DONE
  Fixed with `go:embed` (`embed.go`): templates/migrations/static baked into the binary, `assetFS()` disk-fallback keeps live editing in dev. `views.Load`/`database.Migrate` take an `fs.FS`; robots.txt + static server read from it. release.yml now builds CSS before compiling. Verified: binary runs from a foreign cwd with no asset dirs present.

- [x] **Docker images publish with zero gating** — `docker.yml:3-6` — ✅ DONE
  `docker` job now `needs: verify` (lint+test+build). Same job also fixes the `VERSION=main` string → `main-<sha>`.

- [x] **Backups cover the DB only** — `backup.go:54` — ✅ DONE (DB side); uploads intentionally out of scope
  `integrity_check` now runs on every freshly written backup; one backup is taken immediately at scheduler start. **Uploads are deliberately not tarred by the app** — re-tarring up to 512 MB of audio every interval is a poor fit; uploads live on the data volume and should be captured at the volume/filesystem level (snapshot, restic, etc.). To be documented in README.

- [x] **entrypoint crashloops on PUID/PGID collisions** — `entrypoint.sh:8-13` — ✅ DONE
  Resolves existing uid/gid via `getent group/passwd <id>` and reuses them; recursive chown gated on a `stat -c %u` check.

- [x] **No container healthcheck** — ✅ DONE
  `HEALTHCHECK` added to Dockerfile hitting `/health` (which already pings the DB via `PingContext`).

- [x] **Restore failure can strand app with a closed DB** — `backup.go:151-168`, `admin.go:99` — ✅ DONE
  `Restore` now returns a `restart bool` (true once the DB is closed); the admin handler exits non-zero on a post-close failure so the supervisor restarts against the intact original DB. Covered by `internal/backup/backup_test.go`.

### Ops / config lower-severity
- [x] `linux-amd64` release binary is glibc-dynamic (CGO unset) — ✅ `CGO_ENABLED=0` set in `release.yml` + `Makefile dist`.
- [ ] `SKALD_SECRET_KEY` is dead config (read `config.go:27`, never used). `.env.example` claims "auto-generated if empty" — false. Remove or wire up.
- [ ] `SKALD_DB_TYPE=postgres` silently opens a garbage sqlite path (`database.go:18` hardcodes sqlite driver). Fail fast on `DBType != "sqlite"`.
- [ ] Down migrations are dead code (only `*.up.sql` executed) and internally inconsistent. Add a down-runner or delete them.
- [x] `chown -R /app/data` on every start — ✅ gated on a `stat` check in entrypoint.sh.
- [ ] Unpinned `npm install tailwindcss` (`Dockerfile:6`) → unreproducible CSS. `.dockerignore` misses `.github/`, `Makefile`, screenshots (which ship in runtime image and are publicly served). CI runs tests without `-race`; golangci-lint pinned to `latest`. _(Note: screenshots are now also embedded in the binary via `go:embed static` — consider moving them out of `static/`.)_
- [x] Docker branch builds get `VERSION=main` (`docker.yml:44`) — ✅ now `main-<sha>`.
- [ ] README: add "download docker-compose.yml first" to Quick Start; remove `SKALD_SECRET_KEY` and postgres implications.

### Ops improvements worth adding
- [x] Structured logging (`log/slog`) + `SKALD_LOG_LEVEL` — ✅ DONE (commit adds `internal/logging`, `SKALD_LOG_FORMAT` text|json, slog request-logger, all call sites converted).
- [x] Prometheus `/metrics` — ✅ DONE (hand-rolled text exposition, stdlib only; exposes `skald_last_backup_timestamp_seconds` + uptime/http_requests/goroutines/mem).

---

## P2 — Frontend / UX bugs

> **Status (2026-07-07):** ✅ All five frontend bugs below fixed in commit `f6ee865` (template-parse + renderMarkdown-structure verified). Progressive-enhancement / a11y section below is still open.

- [x] **Prompter unusable on tablet (the stated target device)** — `templates/prompter/show.html` — ✅ DONE
  Controls bar now `flex flex-wrap` with `gap-y-2` so it wraps instead of overflowing; adjust buttons bumped to `px-2.5 py-1 text-sm` and colour swatches to `h-7 w-7` for tappability.

- [x] **Prompter renders markdown as flat text** — ✅ DONE
  Added heading/list/blockquote/hr rules to `.prompter-content` (sizes relative to base font, `currentColor` so they respect the reader's font colour; `h2` gets an underline divider as the segment marker). Jump-list-from-headings deferred to the Features section.

- [x] **Status pipeline repaints as success on server error** — `episodes/show.html` — ✅ DONE (`if (!e.detail.successful) return;`).

- [x] **Kanban drop has no `.catch()`** — `episodes/kanban.html` — ✅ DONE
  Added `.catch(() => reload())`, early-return + reload on `!response.ok`, and count refresh now updates all `.kanban-count` badges (expanded + collapsed).

- [x] **Global submit-disabler misfires** — `base.html` — ✅ DONE
  Bails on `e.defaultPrevented`; disables `e.submitter`; re-enable matches the specific button via `[data-original-text]`.

### Progressive enhancement / a11y (contradicts CLAUDE.md "works without JS")
> **Status (2026-07-07):** ✅ Fixed in commit `4971756`, except the kanban keyboard affordance (deferred as an enhancement).
- [x] Filter selects — ✅ aria-labels + `<noscript>` submit button added on episodes list, kanban, calendar, timeline; admin role select now confirms before submit and restores prior value on cancel.
- [x] Show Notes / Script toggles — ✅ converted to `<details>/<summary>` (work without JS).
- [x] Empty-state CTAs — ✅ gated behind `.CanEdit`/`.IsAdmin` (episodes, guests, sponsorships, shows).
- [ ] Kanban drag has no keyboard path and unreliable touch support (`kanban.html:55`). Add a "move to column" affordance. _(deferred — genuine enhancement, not a quick bug)_
- [x] `dark:bg-gray-750` → ✅ `dark:bg-gray-700` (`calendar.html`).
- [x] Avatar initial byte-slices UTF-8 — ✅ rune-safe `initial` template helper.

---

## Features worth adding (grounded in existing model/templates)

- [ ] **Prompter as a timing tool** — show estimated total/remaining read time + WPM instead of abstract "speed 5" (`prompter/show.html:145`); add restart-to-top button.
- [ ] **Real segment markers + jump list** in the prompter (closes the vestigial MVP item; builds on the markdown-styling fix).
- [ ] **Live markdown preview + `beforeunload` dirty guard** in the script editor (`episodes/edit.html:94`) — script is the core artifact and it's a bare textarea. `POST /preview` reusing `renderMarkdown` with `hx-trigger="keyup changed delay:500ms"`.
- [ ] **Publish-date chips on kanban cards** — tinted red when overdue and not published. Turns the board into a production tool.
- [ ] **Sponsor deadlines on the dashboard** — `drop_date`/`payment_due` already in model, only visible per-sponsor page. Merge into "Upcoming Schedule" (`home.html:85`). Missed paid ad-read = real money.

---

## Cross-cutting / structural

- [ ] **No tests exist at all** (zero `*_test.go`). Three RBAC bugs above (asset download, episode reassign, guest/sponsor scoping) are exactly what httptest integration tests over the auth matrix would catch. Start there + a migration round-trip test on a temp DB.
- [x] **Transactional episode-save service** — ✅ DONE. `EpisodeStore.Save(ep, EpisodeLinks)` writes the row + tags/guests/hosts/sponsors in one transaction (`internal/models/tx.go` `withTx` + `dbtx`; each `Set*` refactored into a tx-aware core with the public method as a thin wrapper). Create and Update both call `Save`, so the row+links are atomic and the previously-swallowed errors in Create now propagate. Dead `LinkGuest` removed. Tests: `TestEpisodeSaveIsAtomic` (rollback on FK failure) + handler integration tests `TestEpisodeUpdatePersistsAllLinks` / `TestEpisodeCreateInheritsShowHosts` (first handler-level tests in the repo).
- [x] **Shared upload helper** — ✅ DONE. `saveUpload(r, uploadSpec)` in `upload.go` centralizes the fixed-name file save (validate ext → mkdir → remove-old-if-different → copy → return uploads-relative path); `errBadUploadType` lets callers map a disallowed extension to 400 consistently. Applied to episode/show/guest artwork, sponsor order doc, and site logo (5 handlers). Error handling is now uniform: guests no longer 500 on a bad type and sponsorships no longer silently swallow failures. ~224 lines of duplication → one 67-line helper. Tests: `TestSaveUpload` (no-file / bad-ext / save+replace-removes-old). (The generic asset attachment in `assets.go` keeps its own path — different semantics: any file type, original filename, size/content-type tracked.)
- [ ] **Central render/error helpers** — buffered render, generic 500 page, logged detail; kills the `err.Error()` leakage pattern in one place.
- [ ] **Consistent authorization strategy** — scoping is enforced ad hoc per handler. A per-entity "can view/edit" check (guest/sponsorship/asset → via episode → show) would have prevented three findings.

---

## Performance (cheap wins, evidence-based)

- [ ] Middleware runs `users.Count()` on **every request** for first-run detection (`middleware.go:18`) + `users.Get` + `ShowIDsForUser`. Cache setup flag in an atomic bool (invalidate on user create).
- [ ] `site_settings` SELECT on every page render for logo path (`views.go:274`, `main.go:97`). Cache, invalidate on logo update.
- [ ] Calendar/timeline/dashboard load every episode ever and filter by month in Go (`calendar.go:58`, `dashboard.go:42`). Add publish-date range to `EpisodeFilter`; dashboard counts can use `CountByStatus`.
- [ ] Missing reverse-lookup indexes: `episode_guests(guest_id)` (`guest.go:236`), `episode_sponsorships(sponsorship_id)` (`sponsorship.go:175`); also `episodes(updated_at)` for default `List` ordering.
- [ ] Admin users page N+1 — `ShowIDsForUser` per user (`admin.go:135`). One `SELECT user_id, show_id FROM user_shows` does it.

---

## Ruled out (checked, not bugs)

- CSRF is properly applied globally via nosurf ahead of all mounts, including `/auth`.
- Markdown is safe — goldmark default mode, no `WithUnsafe`; raw HTML omitted, `javascript:` URLs filtered.
- Multipart filenames can't traverse — `Part.FileName()` applies `filepath.Base`.
- Backup download/restore paths are `filepath.Base`'d, require `.db`, run `integrity_check`, admin-only.
- No SQL injection — placeholders throughout, including `VACUUM INTO ?`.
- No register-as-admin — `Register` hardcodes "viewer" + gated by `openRegistration`; `Setup` only works while `users.Count()==0`.
- Session fixation handled — `RenewToken` on login/register/setup; logout destroys session; 32-byte crypto/rand tokens.
- Stale authz after role change — `LoadUser` reloads user + show IDs each request.
