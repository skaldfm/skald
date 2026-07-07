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
  **Applied:** added `serverError(w, r, err)` helper (`internal/handlers/errors.go`) that logs the detail and returns a generic 500; replaced the `http.Error(w, err.Error(), 500)` sites across the handlers, plus the admin restore message. Invalid episode status already returns 400 (validated against `models.Statuses`).
  **⚠️ Correction (follow-up review):** this was **not** exhaustive — `internal/handlers/calendar.go` was never converted and still leaked internal error text at 6 sites. **✅ Now fixed:** all six `http.Error(w, err.Error(), 500)` sites in `calendar.go` (Calendar + Timeline handlers) converted to `serverError(w, r, err)`. No `err.Error()` leaks remain in any handler.

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
- [x] `SKALD_SECRET_KEY` is dead config — ✅ DONE. Field removed entirely from `config.go`; `.env.example` no longer mentions it.
- [x] `SKALD_DB_TYPE=postgres` silently opens a garbage sqlite path — ✅ DONE. `main.go:118` fails fast (`slog.Error` + `os.Exit(1)`) when `cfg.DBType != "sqlite"`, before `database.Open`. (Guard lives in `main.go`, not `database.go`.)
- [x] Down migrations are dead code — ✅ DONE. The `*.down.sql` files were deleted; only `*.up.sql` (001–016) remain.
- [x] `chown -R /app/data` on every start — ✅ gated on a `stat` check in entrypoint.sh.
- [x] Unpinned `npm install tailwindcss` / `.dockerignore` / CI `-race` / golangci-lint `latest` — ✅ DONE. tailwind pinned to `4.1.11` in `package.json`; `.dockerignore` now lists `.github/`, `Makefile`, `docs/`; CI runs `go test -race ./...`; golangci-lint pinned to `v2.12.2`.
- [x] Docker branch builds get `VERSION=main` (`docker.yml:44`) — ✅ now `main-<sha>`.
- [x] README: "download docker-compose.yml first" in Quick Start; `SKALD_SECRET_KEY`/postgres implications removed — ✅ DONE (`README.md:117`).

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

- [x] Middleware runs `users.Count()` on **every request** — ✅ DONE. Replaced with `users.HasAnyUser()` backed by an `atomic.Bool` cache (`middleware.go:18`, `user.go:25,35`).
- [x] `site_settings` SELECT on every page render for logo path — ✅ DONE. Cached via `atomic.Pointer[string]`, invalidated in `Update` (`settings.go:19,53`).
- [ ] Calendar/timeline/dashboard load every episode ever and filter by month in Go (`calendar.go:58`, `dashboard.go:43`). Add publish-date range to `EpisodeFilter`; dashboard counts can use `CountByStatus` (method now exists at `episode.go:260` but is unused). **Still open** — `EpisodeFilter` still has no date range.
- [x] Missing reverse-lookup indexes — ✅ DONE. `migrations/016` adds `episode_guests(guest_id)`, `episode_sponsorships(sponsorship_id)`, `episodes(updated_at)`.
- [x] Admin users page N+1 — ✅ DONE. All assignments loaded in one query (`admin.go:143`).

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

---

# Follow-up review (2026-07-07, second pass)

A fresh deep pass over the post-remediation code (commits `bcf1413..e344b5f`) plus a re-verification of the backlog above. The stale checkboxes and the `err.Error()` correction are folded into the sections above. New findings below.

## New bugs found this pass

- [x] **HIGH — Stored XSS / privilege escalation via generic episode asset upload** — `internal/handlers/assets.go:69`, served at `main.go:270` — ✅ FIXED
  The P0/P1 upload-allowlist fix (`upload.go`) only covered the **fixed-name** uploads via `saveUpload`. The generic episode-asset path in `assets.go` `Upload` was deliberately left out (see line 164 parenthetical) and accepts **any** extension, storing the file under `<data>/uploads/<episodeID>/<original-filename>` — the exact tree served by the authenticated `/uploads/*` `http.FileServer`. An editor uploads `notes.html` (or `.svg`); it becomes reachable at `/uploads/12/notes.html`, and `http.FileServer` sets `Content-Type: text/html` from the extension, so it executes on the app origin for any authenticated visitor. `nosniff` doesn't help (no sniffing involved) and there is no CSP. Escalation: send the link to an admin → script scrapes the CSRF token from `/admin/users` and POSTs a new admin (or hits `/admin/backups/restore`).
  **Fix applied:** added a `forceDownload` wrapper around the `/uploads/*` file server (`main.go`) that sets `Content-Disposition: attachment` on every served upload, so any top-level navigation to an uploaded file downloads instead of rendering — closing the vector for every extension (html, svg, …) at the serving layer rather than blocklisting extensions on the arbitrary-attachment feature. Inline artwork is unaffected because `<img>`/`<link rel=icon>` subresource loads ignore `Content-Disposition` (verified: the only `<a href>` into `/uploads` is the sponsor order-doc "Download" link). Regression-tested in `main_test.go` (`TestUploadsForceDownload`, `TestUploadsNoDirectoryListing`). This also corrects the doc's line 31 claim that the stored-XSS-via-upload vector was already fully closed.

- [x] **MEDIUM — Setup takeover gated on a discarded DB error** — `internal/handlers/auth.go:268`, `middleware.go:18` — ✅ FIXED
  `Setup` guarded with `hasUser, _ := h.users.HasAnyUser()` — the error was discarded. The `hasUser` cache (`user.go:35`) only ever caches `true`, so before the first positive read a transient SQLite error (e.g. `SQLITE_BUSY` past the busy_timeout under the single-conn pool, or the brief restore window) yielded `hasUser=false`, and `Setup` proceeded to create an **admin** account on an already-populated system.
  **Fix applied:** both `Setup` and `SetupForm` now capture the `HasAnyUser` error and fail closed via `serverError` (500) instead of proceeding. The first-run redirect in `LoadUser` was already safe — it only redirects to setup when `err == nil && !hasUser` (i.e. it never treats an error as "no users").

- [x] **MEDIUM — Login rate-limiter trivially bypassed via forwarded headers** — `internal/handlers/ratelimit.go:37`, `main.go:206` — ✅ FIXED
  The limiter keys on `clientIP(r)` from `r.RemoteAddr`, which `middleware.RealIP` set from the client-controlled `X-Forwarded-For` / `X-Real-IP`. `RealIP` was applied **unconditionally**, so a direct-to-internet deployment could send a unique `X-Forwarded-For` per request and defeat the 10/15min throttle.
  **Fix applied:** `middleware.RealIP` is now mounted only when `SKALD_TRUST_PROXY=true` (new config, default **false**). On a direct deployment `r.RemoteAddr` is the real socket peer, so the rate-limit key can't be spoofed; behind a proxy the operator opts in. Documented in `.env.example`.

- [x] **MEDIUM — Episode/show deletion leaks upload files on disk** — `internal/handlers/episodes.go:472`, `shows.go:319` — ✅ FIXED
  `Delete` removed the row (asset rows cascade via FK) but never removed the on-disk upload directories, and deleting a show cascades this for every episode. Combined with backups deliberately not covering uploads, disk quietly filled with orphans.
  **Fix applied:** added `removeEpisodeUploads`/`removeShowUploads` helpers (`upload.go`). The episode delete handler removes both `uploads/episodes/{id}` (artwork) and `uploads/{id}` (generic assets) after a successful DB delete. The show delete handler enumerates the show's episodes *before* the cascade delete, then removes each episode's upload dirs plus `uploads/shows/{id}`. Best-effort (post-commit), unit-tested (`TestRemoveEpisodeUploads`/`TestRemoveShowUploads`).

- [x] **LOW — `/metrics` is unauthenticated; "firewall the port" doesn't match the topology** — `main.go:230` — ✅ FIXED
  Unlike `/health` (ok/503 only), `/metrics` exposed `skald_last_backup_timestamp_seconds`, uptime, request counts, goroutines and heap to any unauthenticated caller, and the single-port topology has no separate metrics port to firewall.
  **Fix applied:** optional `SKALD_METRICS_TOKEN` (new config). When set, `/metrics` requires `Authorization: Bearer <token>` (constant-time compare, 401 otherwise); when empty it stays open like `/health` for backward compatibility. Verified end-to-end: no/wrong token → 401, correct token → 200, `/health` unaffected.

- [x] **LOW — `Restore` never prunes its `pre-restore` safety backups** — `backup.go:167`, `admin.go:96` — ✅ FIXED
  Every `Restore` ran `Create("pre-restore")` but never `Prune`, and the process `os.Exit`s before the scheduler prunes, so repeated restores could push good scheduled backups past the retention window.
  **Fix applied:** `Restore` now calls `Prune` (best-effort, logged on failure) immediately after the safety backup is created, bounding total backups to the retention count across repeated restores. Unit-tested (`TestRestorePrunesSafetyBackups`).

### Verified clean this pass (no action)
`models/tx.go` + `episode.go` `Save()` (correct rollback/commit, one tx over row + 4 link tables); `saveUpload`/`uploadSpec` (no path traversal — `Base` constant, `ext` from allowlist, subdir from numeric IDs); `logging.go`; the `atomic.Pointer`/`atomic.Bool` caches (race-free, correctly invalidated); backup `Restore` restart contract; `embed.go` `assetFS` disk-fallback; and the touched templates (`base.html` submitter fix, `kanban.html` `.catch()` + dual-badge refresh, `prompter/show.html` markdown CSS, `renderMarkdown` → goldmark default escaping). Note: standalone `SetEpisodeTags`/`SetEpisodeGuests`/`SetEpisodeSponsorships` wrappers are now dead code (only `Save` is called) — cleanliness, not a bug.

## Features worth adding — expanded (ranked by value/effort)

The original five (prompter timing/WPM, segment jump list, markdown preview + dirty guard, kanban publish-date chips, sponsor deadlines on dashboard) are **all still unimplemented**, verified against the templates. Additional gaps, grounded in the existing model/handlers:

**Tier 1 — cheap workflow wins**
- [ ] **Search over scripts + show notes** — search is `title/description LIKE` only (`episode.go:81`); the core artifact (the script) is unsearchable. SQLite FTS5 over title/description/script/show_notes is nearly free.
- [ ] **Tag filtering — tags are currently write-only.** Tags create/display fine but `EpisodeFilter` has no tag field, `episodes/index.html` has no tag filter, and chips link nowhere. Add `filter.Tag` join + link chips to `/episodes?tag=X`.
- [ ] **Episode duplication** — `POST /episodes/{id}/duplicate`, composable from `Save(ep, EpisodeLinks)`. Weekly-format podcasters retype structure every episode.
- [ ] **List-view sorting + publish-date column + pagination** — README claims "sortable" but `episode.go:86` hardcodes `ORDER BY updated_at DESC`, no sort param, no `LIMIT` anywhere (every list loads all rows), and the list doesn't even show publish date. Build it or fix the README claim.
- [ ] **Record/session date on episodes** — only `PublishDate` exists (`episode.go:22`); calendar/timeline can't show when recording happens. Add `record_date` (migration 017) + second color on calendar/timeline.
- [ ] **Archive shows / age-out published episodes** — no archived flag anywhere; retired shows pollute every dropdown and the kanban Published column grows unbounded. Add `shows.archived` + cap the Published column.

**Tier 2 — data lifecycle & money**
- [ ] **Data export (JSON/CSV)** — only export today is the raw SQLite backup; poor fit for a "your data" pitch. Admin JSON export (full) + per-show episode CSV; all queries already exist.
- [ ] **Sponsorship lifecycle status (lead→booked→aired→invoiced→paid) + per-episode ad placement** — `Sponsorship` tracks the deal but not payment state; `EpisodeSponsor` is a bare join (no pre/mid/post-roll or spot count). This is the feature that loses indie podcasters real money.
- [ ] **Guest outreach status** — `EpisodeGuest` has only `role`; `Guest` has no notes. Add `status` on `episode_guests` (invited→confirmed→recorded→thanked) + `notes` on guests.
- [ ] **Orphaned-upload cleanup + storage view** — pairs with the deletion-leak bug above; add an admin "storage" section that walks `data/uploads`, diffs against asset/artwork paths, shows total + orphans.
- [ ] **iCal feed** (`GET /calendar.ics`, per-user token) — makes Skald's schedule ambient in the podcaster's real calendar.

**Tier 3 — recording session & operators**
- [ ] **Prompter rewind/back-10s + elapsed timer** — arrow keys currently change *speed* only; no recovery from a flub, no pacing clock. Pure JS in the existing script block.
- [ ] **Status-change webhook** (ntfy/Slack/generic POST) — the idiomatic self-hoster integration; far cheaper than an API. Site-setting URL + goroutine POST on status change.
- [ ] **Activity/audit log** — no history beyond `updated_at`; add an `events(entity, entity_id, user_id, action, at)` table (also the data source for the webhook). Useful now that RBAC has shipped.

**v2 candidates (from the explicitly-out-of-v1 list)**
- [ ] **RSS feed generation** — highest-value v2 item; the model already holds ~90% of a valid feed. Add enclosure URL + `GET /shows/{id}/feed.xml` to turn Skald from planner into publisher.
- [ ] Read-only API — second pick, but only after webhooks deliver most of the automation value.
- Note: **multi-user shipped** (RBAC, admin pages) despite being "out of v1" — CLAUDE.md/README framing should catch up. Analytics / audio / social / AI remain correctly out of scope.
