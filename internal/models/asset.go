package models

import (
	"database/sql"
	"fmt"
	"time"
)

var AssetTypes = []string{"script", "audio", "artwork", "notes", "other"}

type Asset struct {
	ID        int64
	EpisodeID int64
	Filename  string
	Filepath  string
	Filetype  string
	Filesize  int64
	AssetType string
	CreatedAt time.Time
}

type AssetStore struct {
	db *sql.DB
}

func NewAssetStore(db *sql.DB) *AssetStore {
	return &AssetStore{db: db}
}

func (s *AssetStore) ListForEpisode(episodeID int64) ([]Asset, error) {
	rows, err := s.db.Query(`SELECT id, episode_id, filename, filepath, filetype, filesize, asset_type, created_at
		FROM assets WHERE episode_id = ? ORDER BY created_at DESC`, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing assets for episode %d: %w", episodeID, err)
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		var a Asset
		if err := rows.Scan(&a.ID, &a.EpisodeID, &a.Filename, &a.Filepath, &a.Filetype, &a.Filesize, &a.AssetType, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

func (s *AssetStore) Get(id int64) (*Asset, error) {
	var a Asset
	err := s.db.QueryRow(`SELECT id, episode_id, filename, filepath, filetype, filesize, asset_type, created_at
		FROM assets WHERE id = ?`, id).Scan(
		&a.ID, &a.EpisodeID, &a.Filename, &a.Filepath, &a.Filetype, &a.Filesize, &a.AssetType, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting asset %d: %w", id, err)
	}
	return &a, nil
}

func (s *AssetStore) Create(episodeID int64, filename, filepath, filetype string, filesize int64, assetType string) (*Asset, error) {
	if assetType == "" {
		assetType = "other"
	}
	result, err := s.db.Exec(`INSERT INTO assets (episode_id, filename, filepath, filetype, filesize, asset_type)
		VALUES (?, ?, ?, ?, ?, ?)`, episodeID, filename, filepath, filetype, filesize, assetType)
	if err != nil {
		return nil, fmt.Errorf("creating asset: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return s.Get(id)
}

func (s *AssetStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM assets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting asset %d: %w", id, err)
	}
	return nil
}
