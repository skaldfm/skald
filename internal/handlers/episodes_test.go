package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/skaldfm/skald/internal/auth"
	"github.com/skaldfm/skald/internal/database"
	"github.com/skaldfm/skald/internal/models"
)

func newHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"), dir)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db, os.DirFS(filepath.Join("..", "..", "migrations"))); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return db
}

func newEpisodeHandler(t *testing.T, db *sql.DB) *EpisodeHandler {
	t.Helper()
	return NewEpisodeHandler(
		models.NewEpisodeStore(db), models.NewShowStore(db), models.NewAssetStore(db),
		models.NewGuestStore(db), models.NewTagStore(db), models.NewSponsorshipStore(db),
		t.TempDir(),
	)
}

// adminRequest builds a multipart POST carrying the given fields as an admin
// user, with an optional chi "id" route param.
func adminRequest(fields map[string][]string, id string) *http.Request {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, values := range fields {
		for _, v := range values {
			_ = mw.WriteField(name, v)
		}
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/episodes", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	ctx := auth.WithUser(req.Context(), &models.User{ID: 1, Role: "admin"})
	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	}
	return req.WithContext(ctx)
}

func mustExecH(t *testing.T, db *sql.DB, q string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(q, args...)
	if err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestEpisodeUpdatePersistsAllLinks drives the Update handler and checks that the
// row plus every association table (tags, guests, hosts, sponsors) is written.
func TestEpisodeUpdatePersistsAllLinks(t *testing.T) {
	db := newHandlerTestDB(t)
	h := newEpisodeHandler(t, db)

	show := mustExecH(t, db, `INSERT INTO shows (name) VALUES ('S')`)
	ep := mustExecH(t, db, `INSERT INTO episodes (show_id, title, status) VALUES (?, 'orig', 'idea')`, show)
	guest := mustExecH(t, db, `INSERT INTO guests (name) VALUES ('G')`)
	host := mustExecH(t, db, `INSERT INTO guests (name, is_host) VALUES ('H', 1)`)
	sponsor := mustExecH(t, db, `INSERT INTO sponsorships (name) VALUES ('Sp')`)

	req := adminRequest(map[string][]string{
		"title":           {"updated"},
		"status":          {"scripted"},
		"show_id":         {itoa(show)},
		"tags":            {"news, weekly"},
		"guest_ids":       {itoa(guest)},
		"host_ids":        {itoa(host)},
		"sponsorship_ids": {itoa(sponsor)},
	}, itoa(ep))
	rr := httptest.NewRecorder()

	h.Update(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body=%s", rr.Code, rr.Body.String())
	}

	var title, status string
	if err := db.QueryRow(`SELECT title, status FROM episodes WHERE id = ?`, ep).Scan(&title, &status); err != nil {
		t.Fatalf("reloading episode: %v", err)
	}
	if title != "updated" || status != "scripted" {
		t.Errorf("row = (%q,%q), want (updated,scripted)", title, status)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM episode_tags WHERE episode_id = ?`, ep, 2)
	assertCount(t, db, `SELECT COUNT(*) FROM episode_guests WHERE episode_id = ? AND role = 'guest'`, ep, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM episode_guests WHERE episode_id = ? AND role = 'host'`, ep, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM episode_sponsorships WHERE episode_id = ?`, ep, 1)
}

// TestEpisodeCreateInheritsShowHosts checks the Create path persists the episode
// and auto-inherits the show's hosts.
func TestEpisodeCreateInheritsShowHosts(t *testing.T) {
	db := newHandlerTestDB(t)
	h := newEpisodeHandler(t, db)

	show := mustExecH(t, db, `INSERT INTO shows (name) VALUES ('S')`)
	host := mustExecH(t, db, `INSERT INTO guests (name, is_host) VALUES ('H', 1)`)
	mustExecH(t, db, `INSERT INTO show_hosts (show_id, guest_id) VALUES (?, ?)`, show, host)

	req := adminRequest(map[string][]string{
		"title":   {"Pilot"},
		"status":  {"idea"},
		"show_id": {itoa(show)},
		"tags":    {"intro"},
	}, "")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body=%s", rr.Code, rr.Body.String())
	}

	var epID int64
	if err := db.QueryRow(`SELECT id FROM episodes WHERE title = 'Pilot'`).Scan(&epID); err != nil {
		t.Fatalf("episode not created: %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM episode_tags WHERE episode_id = ?`, epID, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM episode_guests WHERE episode_id = ? AND role = 'host'`, epID, 1)
}

func assertCount(t *testing.T, db *sql.DB, query string, arg any, want int) {
	t.Helper()
	var n int
	if err := db.QueryRow(query, arg).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	if n != want {
		t.Errorf("count for %q = %d, want %d", query, n, want)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
