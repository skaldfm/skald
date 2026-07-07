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

> **Status (2026-07-07, second pass):** most of P1 landed and was build/vet/smoke-verified. Two items are intentionally **deferred**: the transactional episode-save (multi-store refactor, do deliberately) and the guest/sponsor scoping (needs a product decision). See the ⏸ markers.

- [x] **Upload file type never validated** — fixed
  **Applied:** added `internal/handlers/upload.go` with image/doc extension allowlists (SVG excluded — scriptable); applied to episode/show artwork, guest image, sponsor order file; admin logo now uses the shared helper (drops `.svg`). Combined with `X-Content-Type-Options: nosniff` (added globally) this closes the stored-XSS-via-upload vector.

- [x] **Episode reassignment escapes scope** — `handlers/episodes.go` → fixed
  **Applied:** `Update` now checks `CanAccessShow` on the target show before accepting a `show_id` change.

- [ ] ⏸ **Guest & sponsorship detail/edit ignore show scoping** — `guests.go`, `sponsorships.go` (+ Edit/Update/Delete) — **DEFERRED, decision needed**
  Lists are scoped via `ListByShowIDs`, but `Show`/`Edit`/`Update`/`Delete` fetch by ID with no scope check. Restricted users can enumerate IDs to read all guest contacts and sponsor CPM/total-cost; any editor can edit/delete them.
  **Decision needed:** are guests/sponsors instance-global (shared across shows) or show-scoped? The data model links them to episodes (many shows), suggesting global — but then sponsor financials leak across tenants. This also gates full per-show scoping of `/uploads`.

- [x] **Session & CSRF cookies hardcoded `Secure=false`** — fixed
  **Applied:** driven by `SKALD_SECURE_COOKIES` (default **true**); set `false` for plain-HTTP LAN access without a TLS proxy. Verified the CSRF cookie now carries `Secure`.

- [x] **Login user-enumeration (timing)** — `auth.go` → fixed
  **Applied:** `CheckDummyPassword` runs a bcrypt compare against a fixed hash when the account doesn't exist, equalizing response time. (The explicit "email already exists" register message is a minor secondary leak, left as-is.)

### Hardening (not bugs)
- [x] Security headers middleware — `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, HSTS when secure. **CSP intentionally omitted** (templates use inline `<script>`/`onclick`; needs a template refactor first).
- [ ] Idle/absolute session timeout — still a flat 30-day lifetime, no idle expiry.
- [ ] Password max length + login rate-limit/lockout — bcrypt's lib already errors on >72 bytes; explicit max-length message and brute-force lockout still not added.

---

## P1 — Correctness & data safety

- [x] **`SKALD_BACKUP_INTERVAL=0` panics the process** — `backup.go` → fixed
  **Applied:** `StartSchedule` returns early (logs "disabled") on a non-positive interval. Verified: boots cleanly with `SKALD_BACKUP_INTERVAL=0`.

- [ ] ⏸ **Episode save is non-transactional, all errors swallowed** — `episodes.go` — **DEFERRED (refactor)**
  Tags/guests/hosts/sponsors synced via individual autocommit statements, every error discarded (`_ =`). Needs a transaction threaded through the guest/tag/sponsorship stores (they each hold `*sql.DB`) — a deliberate multi-file change, not a quick edit. Partially mitigated already: `SetMaxOpenConns(1)` (P0) removes the `SQLITE_BUSY` trigger that was the most likely cause of silent drops.
  **Fix:** `EpisodeStore.SaveWithLinks(tx, ...)` or pass a `*sql.Tx` into the link stores; propagate errors.

- [ ] **Multipart size limits are dead code** — **DEFERRED (needs decision)**
  A global `MaxBytesReader` would break large **audio** asset uploads (podcast files are big). Needs a configurable max-upload size (e.g. `SKALD_MAX_UPLOAD`) applied per-route, not a blanket cap.

- [x] **Expired sessions never cleaned up** — `main.go` → fixed
  **Applied:** `sessionStore.Cleanup(time.Hour)` started at boot.

- [x] **No graceful shutdown or server timeouts** — `main.go` → fixed
  **Applied:** `http.Server` with `ReadHeaderTimeout`/`IdleTimeout`, `signal.NotifyContext(SIGINT/SIGTERM)` + `srv.Shutdown`. Verified clean exit on SIGTERM. `/health` now does `db.PingContext` (returns 503 on a dead DB).

- [ ] **Internal error text leaked to browsers** — **partially done**
  `http.Error(w, err.Error(), 500)` still used in most handlers (info disclosure). **Done:** invalid episode status now returns 400 (validated against `models.Statuses` in Create/Update/UpdateStatus) instead of a raw CHECK-constraint 500.
  **Remaining:** central error helper (log detail, return generic message) applied across handlers.

### Lower-severity correctness
- [ ] Episode-number uniqueness check-then-write race + NULL season loophole — `episodes.go:163-185`, `003_unique_episode_number.up.sql`.
- [ ] `migrate.go:42` treats any query error (not just `ErrNoRows`) as "not applied" → re-apply attempt. Use `errors.Is(err, sql.ErrNoRows)`.
- [ ] Ignored `Atoi` errors — `episodes.go:154,158,331,337`: `episode_number=abc` silently stores `0`.
- [ ] `assets.go:94` stores absolute `Filepath` → breaks if data dir moves/restored elsewhere. Store relative paths.
- [ ] `views.Render` writes directly to ResponseWriter (`views.go:295`) → template error mid-render yields a garbled half-page. Render to a buffer first.
- [ ] Dashboard counts unscoped guests — `dashboard.go:48` uses `guests.List()` while the rest of the page is scoped.

---

## P2 — Infra / ops

- [ ] **Release binaries are unrunnable standalone** — `release.yml`, `main.go:50,55,114`
  No `go:embed`; templates/migrations/static read from cwd. Downloaded `skald-linux-amd64` fatals at "Failed to load templates."
  **Fix:** `embed.FS` (disk fallback for dev) or ship tarballs with asset dirs.

- [ ] **Docker images publish with zero gating** — `docker.yml:3-6`
  Builds/pushes `:latest` on every push to main with no dependency on CI. A commit failing lint+tests still ships to users on `pull: latest` + `restart: unless-stopped`.
  **Fix:** gate with `needs:` / `workflow_run` on CI success.

- [ ] **Backups cover the DB only** — `backup.go:54`
  `VACUUM INTO` is correct, but `data/uploads/` is never captured, and restore desyncs DB rows against missing files. Document at minimum; better, tar uploads alongside. Also run `integrity_check` on the fresh backup, and do one backup immediately at scheduler start (currently first scheduled backup is a full interval after boot).

- [ ] **entrypoint crashloops on PUID/PGID collisions** — `entrypoint.sh:8-13`
  Checks by name (`skald`), not uid/gid. `PGID=100` (alpine "users") → `addgroup` fails under `set -e` → crashloop.
  **Fix:** detect existing uid/gid and reuse.

- [ ] **No container healthcheck** despite `/health` existing (`main.go:121`). Also `/health` doesn't ping the DB (stays green with a broken database).
  **Fix:** `HEALTHCHECK CMD wget -qO- http://127.0.0.1:${SKALD_PORT}/health`; make `/health` do `db.Ping()`.

- [ ] **Restore failure can strand app with a closed DB** — `backup.go:151-168`, `admin.go:99`
  After `db.Close()`, if copy/rename fails, process keeps running with a closed DB (every request 500s). Exit on error after Close.

### Ops / config lower-severity
- [ ] `linux-amd64` release binary is glibc-dynamic (CGO unset) while Dockerfile uses `CGO_ENABLED=0`. Set `CGO_ENABLED=0` in `release.yml` + `Makefile`.
- [ ] `SKALD_SECRET_KEY` is dead config (read `config.go:27`, never used). `.env.example` claims "auto-generated if empty" — false. Remove or wire up.
- [ ] `SKALD_DB_TYPE=postgres` silently opens a garbage sqlite path (`database.go:18` hardcodes sqlite driver). Fail fast on `DBType != "sqlite"`.
- [ ] Down migrations are dead code (only `*.up.sql` executed) and internally inconsistent. Add a down-runner or delete them.
- [ ] `chown -R /app/data` on every start (`entrypoint.sh:15`) — O(uploads) startup cost; gate on a stat check.
- [ ] Unpinned `npm install tailwindcss` (`Dockerfile:6`) → unreproducible CSS. `.dockerignore` misses `.github/`, `Makefile`, screenshots (which ship in runtime image and are publicly served). CI runs tests without `-race`; golangci-lint pinned to `latest`.
- [ ] Docker branch builds get `VERSION=main` (`docker.yml:44`) → "Skald main starting". Use `github.sha` fallback.
- [ ] README: add "download docker-compose.yml first" to Quick Start; remove `SKALD_SECRET_KEY` and postgres implications.

### Ops improvements worth adding
- [ ] Structured logging (`log/slog`) + `SKALD_LOG_LEVEL`.
- [ ] Prometheus `/metrics` — Go runtime, HTTP counts, **last-backup-timestamp gauge** (last-backup-age is the one alert a self-hoster needs).

---

## P2 — Frontend / UX bugs

- [ ] **Prompter unusable on tablet (the stated target device)** — `templates/prompter/show.html:6-7`
  ~18 controls in a single non-wrapping centered flex row (~900px) overflow both edges on iPad portrait (768px); touch targets ~24px vs 44px guideline.
  **Fix:** `flex-wrap` (or two rows on `<md`), bigger touch padding.

- [ ] **Prompter renders markdown as flat text** — `prompter/show.html:83,117-120`
  No `prose` class → `# Segment` headings look identical to body, lists lose bullets. Guts the MVP "segment markers" promise.
  **Fix:** style headings/lists/blockquote/hr in `.prompter-content`; build a jump list from headings.

- [ ] **Status pipeline repaints as success on server error** — `episodes/show.html:90-103`
  `htmx:afterRequest` recolors without checking `e.detail.successful`. One-line fix: `if (!e.detail.successful) return;`.

- [ ] **Kanban drop has no `.catch()`** — `kanban.html:196-214`
  Network failure leaves card in wrong column with no feedback; only expanded header count updates (collapsed `.kanban-count` bar goes stale).
  **Fix:** `.catch(() => location.reload())`; update via `querySelectorAll('.kanban-count')`.

- [ ] **Global submit-disabler misfires** — `base.html:27-42`
  (a) Doesn't check `defaultPrevented` → cancelling any `confirm()` sticks the button at "Saving…" forever. (b) Grabs the Delete button instead of Save on edit pages.
  **Fix:** bail on `e.defaultPrevented`; use `e.submitter`.

### Progressive enhancement / a11y (contradicts CLAUDE.md "works without JS")
- [ ] Filter selects use `onchange=submit` with no fallback button, no labels — `episodes/index.html:20`, `kanban.html:11`, `calendar.html:22`, `timeline.html:11`, admin role select (also changes role instantly, no confirm).
- [ ] Show Notes toggle is JS-only (`episodes/show.html:135`) → unreachable without JS. Use `<details>/<summary>`.
- [ ] Empty-state "Create your first…" CTAs not permission-gated → viewers get a link into a 403 (`episodes/index.html:89`, guests/sponsorships/shows index).
- [ ] Kanban drag has no keyboard path and unreliable touch support (`kanban.html:55`). Add a "move to column" affordance.
- [ ] `dark:bg-gray-750` doesn't exist (`calendar.html:42`) → weekday header has no dark bg.
- [ ] Avatar initial byte-slices UTF-8 (`guests/index.html:28`, `{{slice .Name 0 1}}`) → "Östen" → "Ã". Use a rune-safe helper.

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
- [ ] **Transactional episode-save service** (highest-leverage refactor) — ~90 lines of tag/guest/host/sponsor diff-and-sync duplicated across Create/Update; source of the swallowed-error and non-atomic-write bugs. Consolidating fixes several findings at once.
- [ ] **Shared upload helper** — same ~35-line save-file block appears 5×2 with inconsistent error handling. Centralize with extension allowlist + size cap.
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
