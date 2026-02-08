package models

import (
	"database/sql"
	"fmt"
)

type Tag struct {
	ID   int64
	Name string
}

type TagStore struct {
	db *sql.DB
}

func NewTagStore(db *sql.DB) *TagStore {
	return &TagStore{db: db}
}

func (s *TagStore) List() ([]Tag, error) {
	rows, err := s.db.Query(`SELECT id, name FROM tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (s *TagStore) GetOrCreate(name string) (*Tag, error) {
	var t Tag
	err := s.db.QueryRow(`SELECT id, name FROM tags WHERE name = ?`, name).Scan(&t.ID, &t.Name)
	if err == nil {
		return &t, nil
	}

	result, err := s.db.Exec(`INSERT INTO tags (name) VALUES (?)`, name)
	if err != nil {
		return nil, fmt.Errorf("creating tag: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return &Tag{ID: id, Name: name}, nil
}

func (s *TagStore) TagsForEpisode(episodeID int64) ([]Tag, error) {
	rows, err := s.db.Query(`SELECT t.id, t.name FROM tags t
		JOIN episode_tags et ON et.tag_id = t.id
		WHERE et.episode_id = ? ORDER BY t.name`, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing tags for episode %d: %w", episodeID, err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (s *TagStore) SetEpisodeTags(episodeID int64, tagNames []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Clear existing tags
	if _, err := tx.Exec(`DELETE FROM episode_tags WHERE episode_id = ?`, episodeID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clearing tags: %w", err)
	}

	// Add new tags
	for _, name := range tagNames {
		if name == "" {
			continue
		}

		// Get or create tag
		var tagID int64
		err := tx.QueryRow(`SELECT id FROM tags WHERE name = ?`, name).Scan(&tagID)
		if err != nil {
			result, err := tx.Exec(`INSERT INTO tags (name) VALUES (?)`, name)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("creating tag %q: %w", name, err)
			}
			tagID, _ = result.LastInsertId()
		}

		// Link to episode
		if _, err := tx.Exec(`INSERT OR IGNORE INTO episode_tags (episode_id, tag_id) VALUES (?, ?)`, episodeID, tagID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("linking tag %q: %w", name, err)
		}
	}

	return tx.Commit()
}

// Delete removes a tag and its episode links.
func (s *TagStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM tags WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting tag %d: %w", id, err)
	}
	return nil
}
