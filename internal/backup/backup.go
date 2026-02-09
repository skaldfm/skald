package backup

import (
	"database/sql"
	"fmt"
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
	retain    int
}

// NewManager creates a backup manager. retain is how many backups to keep (default 7).
func NewManager(db *sql.DB, dataDir string, retain int) *Manager {
	if retain <= 0 {
		retain = 7
	}
	return &Manager{
		db:        db,
		backupDir: filepath.Join(dataDir, "backups"),
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
