package models

import (
	"database/sql"
	"fmt"
	"time"
)

var Statuses = []string{"idea", "research", "scripted", "recorded", "edited", "published"}

type Episode struct {
	ID            int64
	ShowID        int64
	ShowName      string
	Title         string
	EpisodeNumber *int
	SeasonNumber  *int
	Description   string
	Status        string
	PublishDate   *time.Time
	Script        string
	ShowNotes     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type EpisodeFilter struct {
	ShowID int64
	Status string
	Search string
}

type EpisodeStore struct {
	db *sql.DB
}

func NewEpisodeStore(db *sql.DB) *EpisodeStore {
	return &EpisodeStore{db: db}
}

func (s *EpisodeStore) List(filter EpisodeFilter) ([]Episode, error) {
	query := `SELECT e.id, e.show_id, s.name, e.title, e.episode_number, e.season_number,
		e.description, e.status, e.publish_date, e.created_at, e.updated_at
		FROM episodes e
		JOIN shows s ON s.id = e.show_id
		WHERE 1=1`
	var args []any

	if filter.ShowID > 0 {
		query += " AND e.show_id = ?"
		args = append(args, filter.ShowID)
	}
	if filter.Status != "" {
		query += " AND e.status = ?"
		args = append(args, filter.Status)
	}
	if filter.Search != "" {
		query += " AND (e.title LIKE ? OR e.description LIKE ?)"
		like := "%" + filter.Search + "%"
		args = append(args, like, like)
	}

	query += " ORDER BY e.updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing episodes: %w", err)
	}
	defer rows.Close()

	var episodes []Episode
	for rows.Next() {
		var ep Episode
		if err := rows.Scan(&ep.ID, &ep.ShowID, &ep.ShowName, &ep.Title,
			&ep.EpisodeNumber, &ep.SeasonNumber, &ep.Description,
			&ep.Status, &ep.PublishDate, &ep.CreatedAt, &ep.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning episode: %w", err)
		}
		episodes = append(episodes, ep)
	}
	return episodes, rows.Err()
}

func (s *EpisodeStore) Get(id int64) (*Episode, error) {
	var ep Episode
	err := s.db.QueryRow(`SELECT e.id, e.show_id, s.name, e.title, e.episode_number, e.season_number,
		e.description, e.status, e.publish_date, e.script, e.show_notes,
		e.created_at, e.updated_at
		FROM episodes e
		JOIN shows s ON s.id = e.show_id
		WHERE e.id = ?`, id).Scan(
		&ep.ID, &ep.ShowID, &ep.ShowName, &ep.Title, &ep.EpisodeNumber, &ep.SeasonNumber,
		&ep.Description, &ep.Status, &ep.PublishDate, &ep.Script, &ep.ShowNotes,
		&ep.CreatedAt, &ep.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting episode %d: %w", id, err)
	}
	return &ep, nil
}

func (s *EpisodeStore) Create(showID int64, title, description, status string) (*Episode, error) {
	if status == "" {
		status = "idea"
	}
	result, err := s.db.Exec(`INSERT INTO episodes (show_id, title, description, status) VALUES (?, ?, ?, ?)`,
		showID, title, description, status)
	if err != nil {
		return nil, fmt.Errorf("creating episode: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return s.Get(id)
}

func (s *EpisodeStore) Update(ep *Episode) error {
	_, err := s.db.Exec(`UPDATE episodes SET
		show_id = ?, title = ?, episode_number = ?, season_number = ?,
		description = ?, status = ?, publish_date = ?, script = ?, show_notes = ?,
		updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		ep.ShowID, ep.Title, ep.EpisodeNumber, ep.SeasonNumber,
		ep.Description, ep.Status, ep.PublishDate, ep.Script, ep.ShowNotes,
		ep.ID)
	if err != nil {
		return fmt.Errorf("updating episode %d: %w", ep.ID, err)
	}
	return nil
}

func (s *EpisodeStore) UpdateStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE episodes SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, id)
	if err != nil {
		return fmt.Errorf("updating episode %d status: %w", id, err)
	}
	return nil
}

func (s *EpisodeStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM episodes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting episode %d: %w", id, err)
	}
	return nil
}

// CountByStatus returns a map of status -> count, optionally filtered by show.
func (s *EpisodeStore) CountByStatus(showID int64) (map[string]int, error) {
	query := `SELECT status, COUNT(*) FROM episodes`
	var args []any
	if showID > 0 {
		query += " WHERE show_id = ?"
		args = append(args, showID)
	}
	query += " GROUP BY status"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("counting episodes by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning status count: %w", err)
		}
		counts[status] = count
	}
	return counts, rows.Err()
}
