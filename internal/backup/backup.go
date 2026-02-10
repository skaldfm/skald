package backup

import (
	"database/sql"
	"fmt"
	"io"
	"log"
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

	log.Printf("Backup created: %s", filename)
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
			log.Printf("Failed to remove old backup %s: %v", b.Name, err)
		} else {
			log.Printf("Pruned old backup: %s", b.Name)
		}
	}

	return nil
}

// Restore replaces the live database with a backup file.
// It validates the backup, creates a safety backup, closes the DB, and atomically
// replaces the DB file. The caller should exit the process after this returns.
func (m *Manager) Restore(name string) error {
	// Sanitize filename
	name = filepath.Base(name)
	if !strings.HasSuffix(name, ".db") {
		return fmt.Errorf("invalid backup filename")
	}

	backupPath := filepath.Join(m.backupDir, name)
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Validate the backup is a valid SQLite database
	if err := validateSQLite(backupPath); err != nil {
		return fmt.Errorf("backup validation failed: %w", err)
	}

	// Create a safety backup before doing anything destructive
	safetyName, err := m.Create("pre-restore")
	if err != nil {
		return fmt.Errorf("creating safety backup: %w", err)
	}
	log.Printf("Safety backup created: %s", safetyName)

	// Close the live database connection
	if err := m.db.Close(); err != nil {
		return fmt.Errorf("closing database: %w", err)
	}

	// Remove WAL files to prevent stale WAL from corrupting the restored DB
	os.Remove(m.dbPath + "-wal")
	os.Remove(m.dbPath + "-shm")

	// Atomic replace: copy to temp file, then rename over the live DB
	tmpPath := m.dbPath + ".restoring"
	if err := copyFile(backupPath, tmpPath); err != nil {
		return fmt.Errorf("copying backup: %w", err)
	}

	if err := os.Rename(tmpPath, m.dbPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replacing database: %w", err)
	}

	log.Printf("Database restored from %s", name)
	return nil
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
func (m *Manager) StartSchedule(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := m.Create("scheduled"); err != nil {
				log.Printf("Scheduled backup failed: %v", err)
			}
			if err := m.Prune(); err != nil {
				log.Printf("Backup prune failed: %v", err)
			}
		}
	}()
	log.Printf("Backup scheduler started (every %s, retain %d)", interval, m.retain)
}
