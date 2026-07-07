package backup

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Info describes a single backup file.
type Info struct {
	Name      string
	Size      int64
	CreatedAt time.Time
}

// Manager handles database backups.
type Manager struct {
	db        *sql.DB
	backupDir string
	dbPath    string
	retain    int
}

// NewManager creates a backup manager. retain is how many backups to keep (default 7).
func NewManager(db *sql.DB, dataDir, dbPath string, retain int) *Manager {
	if retain <= 0 {
		retain = 7
	}
	return &Manager{
		db:        db,
		backupDir: filepath.Join(dataDir, "backups"),
		dbPath:    dbPath,
		retain:    retain,
	}
}

// Create makes a new backup with the given label (e.g. "manual", "scheduled", "pre-migration").
// Returns the backup filename.
func (m *Manager) Create(label string) (string, error) {
	if err := os.MkdirAll(m.backupDir, 0755); err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("skald-%s-%s.db", ts, label)
	destPath := filepath.Join(m.backupDir, filename)

	if _, err := m.db.Exec("VACUUM INTO ?", destPath); err != nil {
		return "", fmt.Errorf("creating backup: %w", err)
	}

	// Verify the freshly written backup is a valid, uncorrupted database before
	// we trust it (and before Prune may delete an older good one).
	if err := validateSQLite(destPath); err != nil {
		_ = os.Remove(destPath)
		return "", fmt.Errorf("verifying backup: %w", err)
	}

	slog.Info("backup created", "file", filename)
	return filename, nil
}

// List returns all backups sorted newest first.
func (m *Manager) List() ([]Info, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading backup directory: %w", err)
	}

	var backups []Info
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, Info{
			Name:      e.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// FilePath returns the full path for a backup file.
func (m *Manager) FilePath(name string) string {
	return filepath.Join(m.backupDir, name)
}

// LastBackupTime returns the timestamp of the most recent backup, and false if
// no backups exist yet. Used for the last-backup-age metric.
func (m *Manager) LastBackupTime() (time.Time, bool) {
	backups, err := m.List()
	if err != nil || len(backups) == 0 {
		return time.Time{}, false
	}
	return backups[0].CreatedAt, true // List is sorted newest-first
}

// Prune removes old backups beyond the retention count.
func (m *Manager) Prune() error {
	backups, err := m.List()
	if err != nil {
		return err
	}

	if len(backups) <= m.retain {
		return nil
	}

	for _, b := range backups[m.retain:] {
		path := filepath.Join(m.backupDir, b.Name)
		if err := os.Remove(path); err != nil {
			slog.Warn("failed to remove old backup", "file", b.Name, "err", err)
		} else {
			slog.Info("pruned old backup", "file", b.Name)
		}
	}

	return nil
}

// Restore replaces the live database with a backup file.
// It validates the backup, creates a safety backup, closes the DB, and atomically
// replaces the DB file.
//
// The returned restart flag reports whether the live DB handle has been closed.
// Once it is true the process can no longer serve requests (every query hits a
// closed DB) and the caller MUST exit so it restarts against the new DB file —
// this holds on both success and failure, since the close happens before the
// swap and is not undone on error.
func (m *Manager) Restore(name string) (restart bool, err error) {
	// Sanitize filename
	name = filepath.Base(name)
	if !strings.HasSuffix(name, ".db") {
		return false, fmt.Errorf("invalid backup filename")
	}

	backupPath := filepath.Join(m.backupDir, name)
	if _, err := os.Stat(backupPath); err != nil {
		return false, fmt.Errorf("backup file not found: %w", err)
	}

	// Validate the backup is a valid SQLite database
	if err := validateSQLite(backupPath); err != nil {
		return false, fmt.Errorf("backup validation failed: %w", err)
	}

	// Create a safety backup before doing anything destructive
	safetyName, err := m.Create("pre-restore")
	if err != nil {
		return false, fmt.Errorf("creating safety backup: %w", err)
	}
	slog.Info("safety backup created", "file", safetyName)

	// Close the live database connection. From here on the process must restart
	// regardless of outcome — the DB handle is dead.
	if err := m.db.Close(); err != nil {
		return false, fmt.Errorf("closing database: %w", err)
	}

	// Remove WAL files to prevent stale WAL from corrupting the restored DB
	os.Remove(m.dbPath + "-wal")
	os.Remove(m.dbPath + "-shm")

	// Atomic replace: copy to temp file, then rename over the live DB
	tmpPath := m.dbPath + ".restoring"
	if err := copyFile(backupPath, tmpPath); err != nil {
		return true, fmt.Errorf("copying backup: %w", err)
	}

	if err := os.Rename(tmpPath, m.dbPath); err != nil {
		os.Remove(tmpPath)
		return true, fmt.Errorf("replacing database: %w", err)
	}

	slog.Info("database restored", "from", name)
	return true, nil
}

func validateSQLite(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer db.Close()

	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// StartSchedule runs backups on a fixed interval in a background goroutine.
// A non-positive interval disables scheduled backups (time.NewTicker panics on
// a zero/negative duration).
func (m *Manager) StartSchedule(interval time.Duration) {
	if interval <= 0 {
		slog.Info("scheduled backups disabled", "interval", interval)
		return
	}
	go func() {
		// Take one backup immediately so a crash shortly after boot still leaves a
		// recent copy, rather than waiting a full interval for the first one.
		runBackup := func() {
			if _, err := m.Create("scheduled"); err != nil {
				slog.Error("scheduled backup failed", "err", err)
			}
			if err := m.Prune(); err != nil {
				slog.Error("backup prune failed", "err", err)
			}
		}
		runBackup()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			runBackup()
		}
	}()
	slog.Info("backup scheduler started", "interval", interval, "retain", m.retain)
}
