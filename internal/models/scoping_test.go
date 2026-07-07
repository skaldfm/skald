package models

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/skaldfm/skald/internal/database"
)

// newTestDB opens a fresh temp SQLite database with the real migrations applied.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"), dir)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db, os.DirFS(filepath.Join("..", "..", "migrations"))); err != nil {
		t.Fatalf("migrating test db: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestGuestAccessibleToShows(t *testing.T) {
	db := newTestDB(t)

	show1 := mustExec(t, db, `INSERT INTO shows (name) VALUES ('Show 1')`)
	show2 := mustExec(t, db, `INSERT INTO shows (name) VALUES ('Show 2')`)
	ep1 := mustExec(t, db, `INSERT INTO episodes (show_id, title, status) VALUES (?, 'E1', 'idea')`, show1)
	ep2 := mustExec(t, db, `INSERT INTO episodes (show_id, title, status) VALUES (?, 'E2', 'idea')`, show2)

	gLinked1 := mustExec(t, db, `INSERT INTO guests (name) VALUES ('Linked to show1')`)
	gLinked2 := mustExec(t, db, `INSERT INTO guests (name) VALUES ('Linked to show2')`)
	gOrphan := mustExec(t, db, `INSERT INTO guests (name) VALUES ('Orphan')`)
	gHost1 := mustExec(t, db, `INSERT INTO guests (name, is_host) VALUES ('Host of show1', 1)`)

	mustExec(t, db, `INSERT INTO episode_guests (episode_id, guest_id, role) VALUES (?, ?, 'guest')`, ep1, gLinked1)
	mustExec(t, db, `INSERT INTO episode_guests (episode_id, guest_id, role) VALUES (?, ?, 'guest')`, ep2, gLinked2)
	mustExec(t, db, `INSERT INTO show_hosts (show_id, guest_id) VALUES (?, ?)`, show1, gHost1)

	store := NewGuestStore(db)
	scope := []int64{show1} // a user who can only access show1

	cases := []struct {
		name    string
		guestID int64
		want    bool
	}{
		{"linked to in-scope show", gLinked1, true},
		{"linked only to other show", gLinked2, false},
		{"orphan is visible", gOrphan, true},
		{"host of in-scope show", gHost1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.AccessibleToShows(tc.guestID, scope)
			if err != nil {
				t.Fatalf("AccessibleToShows: %v", err)
			}
			if got != tc.want {
				t.Errorf("AccessibleToShows(%d, %v) = %v, want %v", tc.guestID, scope, got, tc.want)
			}
		})
	}

	// A user assigned to no shows sees only orphans.
	t.Run("no shows sees only orphan", func(t *testing.T) {
		empty := []int64{}
		if ok, _ := store.AccessibleToShows(gLinked1, empty); ok {
			t.Error("linked guest should not be accessible to a user with no shows")
		}
		if ok, _ := store.AccessibleToShows(gOrphan, empty); !ok {
			t.Error("orphan guest should be accessible to a user with no shows")
		}
	})
}

func TestEpisodeNumberUniqueWithNullSeason(t *testing.T) {
	db := newTestDB(t)
	show := mustExec(t, db, `INSERT INTO shows (name) VALUES ('S')`)
	mustExec(t, db, `INSERT INTO episodes (show_id, title, status, episode_number) VALUES (?, 'A', 'idea', 5)`, show)

	// Same show, same episode number, both with NULL season — the migration 015
	// expression index must reject this (SQLite would otherwise treat NULLs as
	// distinct and allow the duplicate).
	if _, err := db.Exec(`INSERT INTO episodes (show_id, title, status, episode_number) VALUES (?, 'B', 'idea', 5)`, show); err == nil {
		t.Fatal("expected a UNIQUE violation for a duplicate episode number with NULL season")
	}

	// A different number, or a season, is still fine.
	mustExec(t, db, `INSERT INTO episodes (show_id, title, status, episode_number) VALUES (?, 'C', 'idea', 6)`, show)
	mustExec(t, db, `INSERT INTO episodes (show_id, title, status, season_number, episode_number) VALUES (?, 'D', 'idea', 1, 5)`, show)
}

func TestSetEpisodeGuestsAndHosts(t *testing.T) {
	db := newTestDB(t)
	show := mustExec(t, db, `INSERT INTO shows (name) VALUES ('Show')`)
	ep := mustExec(t, db, `INSERT INTO episodes (show_id, title, status) VALUES (?, 'E', 'idea')`, show)
	g1 := mustExec(t, db, `INSERT INTO guests (name) VALUES ('G1')`)
	g2 := mustExec(t, db, `INSERT INTO guests (name) VALUES ('G2')`)
	h1 := mustExec(t, db, `INSERT INTO guests (name, is_host) VALUES ('H1', 1)`)

	store := NewGuestStore(db)

	if err := store.SetEpisodeGuests(ep, []int64{g1, g2}); err != nil {
		t.Fatalf("SetEpisodeGuests: %v", err)
	}
	if err := store.SetEpisodeHosts(ep, []int64{h1}); err != nil {
		t.Fatalf("SetEpisodeHosts: %v", err)
	}

	assertIDs := func(label string, got []int64, want ...int64) {
		t.Helper()
		set := map[int64]bool{}
		for _, id := range got {
			set[id] = true
		}
		if len(got) != len(want) {
			t.Errorf("%s = %v, want %v", label, got, want)
			return
		}
		for _, id := range want {
			if !set[id] {
				t.Errorf("%s = %v, missing %d", label, got, id)
			}
		}
	}

	guestIDs, _ := store.GuestIDsForEpisode(ep)
	assertIDs("guests after set", guestIDs, g1, g2)
	hostIDs, _ := store.HostIDsForEpisode(ep)
	assertIDs("hosts after set", hostIDs, h1)

	// Re-setting guests must not disturb hosts.
	if err := store.SetEpisodeGuests(ep, []int64{g2}); err != nil {
		t.Fatalf("SetEpisodeGuests (replace): %v", err)
	}
	guestIDs, _ = store.GuestIDsForEpisode(ep)
	assertIDs("guests after replace", guestIDs, g2)
	hostIDs, _ = store.HostIDsForEpisode(ep)
	assertIDs("hosts unchanged after guest replace", hostIDs, h1)
}

func TestSponsorshipAccessibleToShows(t *testing.T) {
	db := newTestDB(t)

	show1 := mustExec(t, db, `INSERT INTO shows (name) VALUES ('Show 1')`)
	show2 := mustExec(t, db, `INSERT INTO shows (name) VALUES ('Show 2')`)
	ep1 := mustExec(t, db, `INSERT INTO episodes (show_id, title, status) VALUES (?, 'E1', 'idea')`, show1)
	ep2 := mustExec(t, db, `INSERT INTO episodes (show_id, title, status) VALUES (?, 'E2', 'idea')`, show2)

	spLinked1 := mustExec(t, db, `INSERT INTO sponsorships (name) VALUES ('Sponsor of show1')`)
	spLinked2 := mustExec(t, db, `INSERT INTO sponsorships (name) VALUES ('Sponsor of show2')`)
	spOrphan := mustExec(t, db, `INSERT INTO sponsorships (name) VALUES ('Orphan sponsor')`)

	mustExec(t, db, `INSERT INTO episode_sponsorships (episode_id, sponsorship_id) VALUES (?, ?)`, ep1, spLinked1)
	mustExec(t, db, `INSERT INTO episode_sponsorships (episode_id, sponsorship_id) VALUES (?, ?)`, ep2, spLinked2)

	store := NewSponsorshipStore(db)
	scope := []int64{show1}

	cases := []struct {
		name string
		spID int64
		want bool
	}{
		{"linked to in-scope show", spLinked1, true},
		{"linked only to other show", spLinked2, false},
		{"orphan is visible", spOrphan, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.AccessibleToShows(tc.spID, scope)
			if err != nil {
				t.Fatalf("AccessibleToShows: %v", err)
			}
			if got != tc.want {
				t.Errorf("AccessibleToShows(%d, %v) = %v, want %v", tc.spID, scope, got, tc.want)
			}
		})
	}
}
