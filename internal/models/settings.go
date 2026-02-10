package models

import (
	"database/sql"
	"time"
)

type SiteSettings struct {
	ID        int64
	LogoPath  string
	UpdatedAt time.Time
}

type SiteSettingsStore struct {
	db *sql.DB
}

func NewSiteSettingsStore(db *sql.DB) *SiteSettingsStore {
	return &SiteSettingsStore{db: db}
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
	return err
}
