package models

import (
	"database/sql"
	"sync/atomic"
	"time"
)

type SiteSettings struct {
	ID        int64
	LogoPath  string
	UpdatedAt time.Time
}

type SiteSettingsStore struct {
	db *sql.DB
	// logoPath caches the logo path, which is read on every page render but
	// changes only when an admin updates it. nil = not yet loaded.
	logoPath atomic.Pointer[string]
}

func NewSiteSettingsStore(db *sql.DB) *SiteSettingsStore {
	return &SiteSettingsStore{db: db}
}

// LogoPath returns the current site logo path, cached to avoid a site_settings
// read on every page render. Refreshed whenever Update runs.
func (s *SiteSettingsStore) LogoPath() string {
	if p := s.logoPath.Load(); p != nil {
		return *p
	}
	path := ""
	if ss, err := s.Get(); err == nil && ss != nil {
		path = ss.LogoPath
	}
	s.logoPath.Store(&path)
	return path
}

func (s *SiteSettingsStore) Get() (*SiteSettings, error) {
	var ss SiteSettings
	err := s.db.QueryRow(`SELECT id, logo_path, updated_at FROM site_settings WHERE id = 1`).
		Scan(&ss.ID, &ss.LogoPath, &ss.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ss, nil
}

func (s *SiteSettingsStore) Update(logoPath string) error {
	_, err := s.db.Exec(`UPDATE site_settings SET logo_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`, logoPath)
	if err == nil {
		s.logoPath.Store(&logoPath)
	}
	return err
}
