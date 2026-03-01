package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Show struct {
	ID          int64
	Name        string
	Description string
	Artwork     string
	Color       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ShowStore struct {
	db *sql.DB
}

func NewShowStore(db *sql.DB) *ShowStore {
	return &ShowStore{db: db}
}

func (s *ShowStore) List() ([]Show, error) {
	rows, err := s.db.Query(`SELECT id, name, description, artwork, color, created_at, updated_at
		FROM shows ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing shows: %w", err)
	}
	defer rows.Close()

	var shows []Show
	for rows.Next() {
		var show Show
		if err := rows.Scan(&show.ID, &show.Name, &show.Description, &show.Artwork, &show.Color, &show.CreatedAt, &show.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning show: %w", err)
		}
		shows = append(shows, show)
	}
	return shows, rows.Err()
}

func (s *ShowStore) Get(id int64) (*Show, error) {
	var show Show
	err := s.db.QueryRow(`SELECT id, name, description, artwork, color, created_at, updated_at
		FROM shows WHERE id = ?`, id).Scan(
		&show.ID, &show.Name, &show.Description, &show.Artwork, &show.Color, &show.CreatedAt, &show.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting show %d: %w", id, err)
	}
	return &show, nil
}

func (s *ShowStore) Create(name, description, color string) (*Show, error) {
	result, err := s.db.Exec(`INSERT INTO shows (name, description, color) VALUES (?, ?, ?)`, name, description, color)
	if err != nil {
		return nil, fmt.Errorf("creating show: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return s.Get(id)
}

func (s *ShowStore) Update(id int64, name, description, artwork, color string) error {
	_, err := s.db.Exec(`UPDATE shows SET name = ?, description = ?, artwork = ?, color = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		name, description, artwork, color, id)
	if err != nil {
		return fmt.Errorf("updating show %d: %w", id, err)
	}
	return nil
}

func (s *ShowStore) ListByIDs(ids []int64) ([]Show, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := "?" + strings.Repeat(",?", len(ids)-1)
	query := fmt.Sprintf(`SELECT id, name, description, artwork, color, created_at, updated_at
		FROM shows WHERE id IN (%s) ORDER BY name`, placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing shows by IDs: %w", err)
	}
	defer rows.Close()

	var shows []Show
	for rows.Next() {
		var show Show
		if err := rows.Scan(&show.ID, &show.Name, &show.Description, &show.Artwork, &show.Color, &show.CreatedAt, &show.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning show: %w", err)
		}
		shows = append(shows, show)
	}
	return shows, rows.Err()
}

func (s *ShowStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM shows WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting show %d: %w", id, err)
	}
	return nil
}
