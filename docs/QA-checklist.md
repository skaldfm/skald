# Manual QA checklist — v0.4.0 (CSP + kanban a11y)

Automated checks (`go vet`, `go test ./...`, and a scripted server run) already confirmed:
CSP header present on every page, header nonce matches every rendered `<script>`
nonce, zero inline `on*` handlers in rendered HTML, and kanban/prompter render at 200.

What automation **can't** catch is live browser behaviour — under a strict CSP a
missed handler or a broken listener fails silently (the script just doesn't run).
So before cutting anything final, load the app in a real browser with **DevTools →
Console open** and confirm there are **no `Content-Security-Policy` violation errors**
while exercising each item below.

## How to run locally
```fish
# from repo root
make run   # or: go run . with SKALD_SECURE_COOKIES=false for plain-HTTP localhost
# then open http://localhost:7707
```

## Interactive checks (watch the Console for CSP violations throughout)

- [ ] **Dark-mode toggle** (nav on any page, and the toggle on the login page) — flips theme and persists across reload.
- [ ] **Form submit UX** — a Save button shows "Saving…" and disables on submit; re-enables after an HTMX action.
- [ ] **Confirm guards** — deleting an episode / show / guest / sponsorship / user, and the backup **restore**, each show a confirm dialog; Cancel leaves the button untouched (not stuck on "Saving…").
- [ ] **Filter selects auto-submit** — the show/status filters on Episodes, Kanban, Calendar, Timeline reload on change; with JS disabled the `<noscript>` Filter button still works.
- [ ] **File-input labels** — choosing a file on any artwork/logo/order-doc upload updates the filename label next to it.
- [ ] **Publish-date field** (new episode) — the `YYYY-MM-DD` text field switches to a native date picker on focus and back to text when left empty.
- [ ] **Admin → Users** — "Add User" toggles the create form; changing a role prompts to confirm and reverts the select on Cancel.
- [ ] **Kanban drag-and-drop** — drag a card between columns; card moves, counts update, collapsed columns auto-expand on drop; a rejected/failed move reloads the board.
- [ ] **Kanban move-select (new a11y path)** — on a card, use the "move to stage" select via **keyboard only** (Tab to it, change value) and via touch; the card moves and persists exactly like a drag. This is the keyboard/touch affordance added in this release.
- [ ] **Prompter** (`/prompter/{episodeId}`) — Play/Pause, speed ±, font ±, font/bg colour swatches, Center, Mirror, Fullscreen all work via clicks; keyboard shortcuts (space/k, arrows, +/-, f, m, c, Esc) work; auto-scroll runs and progress % updates.
- [ ] **Tag pickers** (episode/show/guest/sponsor forms) — searchable multi-select renders and writes hidden inputs; saving persists selections. (External `tag-picker.js`, allowed by `script-src 'self'`.)
- [ ] **HTMX status pipeline** (episode detail) — changing status via the pipeline updates the badge without a full reload and doesn't paint success on a server error.
- [ ] **Images still render** — artwork, avatars, and the site logo display inline (they load as `<img>` subresources, unaffected by the uploads `Content-Disposition: attachment`).

## Config knobs added this release (spot-check as needed)
- `SKALD_TRUST_PROXY` (default false) — only set true behind a reverse proxy; gates `X-Forwarded-For` trust for the login rate-limiter.
- `SKALD_METRICS_TOKEN` — when set, `/metrics` requires `Authorization: Bearer <token>`; empty leaves it open like `/health`.

## If a CSP violation appears
The offending inline script/handler was missed. Fix by moving it to a nonce'd
`<script nonce="{{.Nonce}}">` block or a delegated `addEventListener` (see the
patterns in `templates/layouts/base.html`). Do **not** relax `script-src` to
`'unsafe-inline'`.
