package backup

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/skaldfm/skald/internal/database"
)

func openDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := database.Open(path, filepath.Dir(path))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	return db
}

func countRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&n); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	return n
}

// TestCreateAndRestore exercises the full backup lifecycle: a verified backup is
// written, later changes are made, and Restore rolls the live DB back to the
// backed-up state while signalling that the process must restart.
func TestCreateAndRestore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "skald.db")

	db := openDB(t, dbPath)
	if _, err := db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("creating table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO items DEFAULT VALUES"); err != nil {
		t.Fatalf("seeding row: %v", err)
	}

	m := NewManager(db, dir, dbPath, 7)

	name, err := m.Create("manual")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Create must leave a verified, listable backup behind.
	if _, err := m.List(); err != nil {
		t.Fatalf("List: %v", err)
	}

	// Mutate the live DB after the backup point.
	if _, err := db.Exec("INSERT INTO items DEFAULT VALUES"); err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if n := countRows(t, db); n != 2 {
		t.Fatalf("expected 2 rows before restore, got %d", n)
	}

	restart, err := m.Restore(name)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !restart {
		t.Fatal("Restore should report restart=true once the DB has been closed")
	}

	// Restore closes the live handle; reopen and confirm we're back to the
	// single row that existed when the backup was taken.
	db2 := openDB(t, dbPath)
	defer db2.Close()
	if n := countRows(t, db2); n != 1 {
		t.Fatalf("expected 1 row after restore, got %d", n)
	}
}

// TestRestoreRejectsBadName guards the filename validation without closing the DB.
func TestRestoreRejectsBadName(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "skald.db")
	db := openDB(t, dbPath)
	defer db.Close()

	m := NewManager(db, dir, dbPath, 7)

	restart, err := m.Restore("not-a-backup.txt")
	if err == nil {
		t.Fatal("expected error for non-.db filename")
	}
	if restart {
		t.Fatal("restart must be false when the DB was never closed")
	}
}
