package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Sponsorship struct {
	ID             int64
	Name           string
	Description    string
	Script         string
	CPM            *float64
	AverageListens *int
	TotalCost      *float64
	DropDate       *time.Time
	PaymentDueDate *time.Time
	OrderFile      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type EpisodeSponsor struct {
	EpisodeID       int64
	SponsorshipID   int64
	SponsorshipName string
	EpisodeTitle    string
}

type SponsorshipStore struct {
	db *sql.DB
}

func NewSponsorshipStore(db *sql.DB) *SponsorshipStore {
	return &SponsorshipStore{db: db}
}

func (s *SponsorshipStore) List() ([]Sponsorship, error) {
	rows, err := s.db.Query(`SELECT id, name, description, script, cpm, average_listens, total_cost,
		drop_date, payment_due_date, order_file, created_at, updated_at
		FROM sponsorships ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing sponsorships: %w", err)
	}
	defer rows.Close()

	var sponsorships []Sponsorship
	for rows.Next() {
		var sp Sponsorship
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.Description, &sp.Script, &sp.CPM, &sp.AverageListens,
			&sp.TotalCost, &sp.DropDate, &sp.PaymentDueDate, &sp.OrderFile, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning sponsorship: %w", err)
		}
		sponsorships = append(sponsorships, sp)
	}
	return sponsorships, rows.Err()
}

// ListByShowIDs returns sponsorships linked to episodes in the given shows.
func (s *SponsorshipStore) ListByShowIDs(showIDs []int64) ([]Sponsorship, error) {
	if len(showIDs) == 0 {
		return nil, nil
	}
	placeholders := "?" + strings.Repeat(",?", len(showIDs)-1)
	query := fmt.Sprintf(`SELECT DISTINCT sp.id, sp.name, sp.description, sp.script, sp.cpm,
		sp.average_listens, sp.total_cost, sp.drop_date, sp.payment_due_date,
		sp.order_file, sp.created_at, sp.updated_at
		FROM sponsorships sp
		WHERE sp.id IN (
			SELECT es.sponsorship_id FROM episode_sponsorships es
			JOIN episodes e ON e.id = es.episode_id
			WHERE e.show_id IN (%s)
		)
		ORDER BY sp.name`, placeholders)

	args := make([]any, 0, len(showIDs))
	for _, id := range showIDs {
		args = append(args, id)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing sponsorships by show IDs: %w", err)
	}
	defer rows.Close()

	var sponsorships []Sponsorship
	for rows.Next() {
		var sp Sponsorship
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.Description, &sp.Script, &sp.CPM, &sp.AverageListens,
			&sp.TotalCost, &sp.DropDate, &sp.PaymentDueDate, &sp.OrderFile, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning sponsorship: %w", err)
		}
		sponsorships = append(sponsorships, sp)
	}
	return sponsorships, rows.Err()
}

func (s *SponsorshipStore) Get(id int64) (*Sponsorship, error) {
	var sp Sponsorship
	err := s.db.QueryRow(`SELECT id, name, description, script, cpm, average_listens, total_cost,
		drop_date, payment_due_date, order_file, created_at, updated_at
		FROM sponsorships WHERE id = ?`, id).Scan(
		&sp.ID, &sp.Name, &sp.Description, &sp.Script, &sp.CPM, &sp.AverageListens,
		&sp.TotalCost, &sp.DropDate, &sp.PaymentDueDate, &sp.OrderFile, &sp.CreatedAt, &sp.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting sponsorship %d: %w", id, err)
	}
	return &sp, nil
}

func (s *SponsorshipStore) Create(name string) (*Sponsorship, error) {
	result, err := s.db.Exec(`INSERT INTO sponsorships (name) VALUES (?)`, name)
	if err != nil {
		return nil, fmt.Errorf("creating sponsorship: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return s.Get(id)
}

func (s *SponsorshipStore) Update(sp *Sponsorship) error {
	_, err := s.db.Exec(`UPDATE sponsorships SET name = ?, description = ?, script = ?,
		cpm = ?, average_listens = ?, total_cost = ?, drop_date = ?, payment_due_date = ?,
		order_file = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		sp.Name, sp.Description, sp.Script, sp.CPM, sp.AverageListens,
		sp.TotalCost, sp.DropDate, sp.PaymentDueDate, sp.OrderFile, sp.ID)
	if err != nil {
		return fmt.Errorf("updating sponsorship %d: %w", sp.ID, err)
	}
	return nil
}

func (s *SponsorshipStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM sponsorships WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting sponsorship %d: %w", id, err)
	}
	return nil
}

func (s *SponsorshipStore) SponsorshipsForEpisode(episodeID int64) ([]EpisodeSponsor, error) {
	rows, err := s.db.Query(`SELECT es.episode_id, es.sponsorship_id, sp.name
		FROM episode_sponsorships es
		JOIN sponsorships sp ON sp.id = es.sponsorship_id
		WHERE es.episode_id = ?
		ORDER BY sp.name`, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing sponsorships for episode %d: %w", episodeID, err)
	}
	defer rows.Close()

	var links []EpisodeSponsor
	for rows.Next() {
		var es EpisodeSponsor
		if err := rows.Scan(&es.EpisodeID, &es.SponsorshipID, &es.SponsorshipName); err != nil {
			return nil, fmt.Errorf("scanning episode sponsorship: %w", err)
		}
		links = append(links, es)
	}
	return links, rows.Err()
}

// EpisodesForSponsorship returns episode links for a sponsorship.
// If showIDs is nil, returns all; otherwise scopes to the given shows.
func (s *SponsorshipStore) EpisodesForSponsorship(sponsorshipID int64, showIDs []int64) ([]EpisodeSponsor, error) {
	query := `SELECT es.episode_id, es.sponsorship_id, e.title
		FROM episode_sponsorships es
		JOIN episodes e ON e.id = es.episode_id
		WHERE es.sponsorship_id = ?`
	args := []any{sponsorshipID}
	if showIDs != nil {
		if len(showIDs) == 0 {
			return nil, nil
		}
		query += " AND e.show_id IN (?" + strings.Repeat(",?", len(showIDs)-1) + ")"
		for _, id := range showIDs {
			args = append(args, id)
		}
	}
	query += " ORDER BY e.created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing episodes for sponsorship %d: %w", sponsorshipID, err)
	}
	defer rows.Close()

	var links []EpisodeSponsor
	for rows.Next() {
		var es EpisodeSponsor
		if err := rows.Scan(&es.EpisodeID, &es.SponsorshipID, &es.EpisodeTitle); err != nil {
			return nil, fmt.Errorf("scanning episode sponsorship: %w", err)
		}
		links = append(links, es)
	}
	return links, rows.Err()
}

func (s *SponsorshipStore) LinkEpisode(sponsorshipID, episodeID int64) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO episode_sponsorships (episode_id, sponsorship_id) VALUES (?, ?)`,
		episodeID, sponsorshipID)
	if err != nil {
		return fmt.Errorf("linking sponsorship %d to episode %d: %w", sponsorshipID, episodeID, err)
	}
	return nil
}

func (s *SponsorshipStore) UnlinkEpisode(sponsorshipID, episodeID int64) error {
	_, err := s.db.Exec(`DELETE FROM episode_sponsorships WHERE episode_id = ? AND sponsorship_id = ?`,
		episodeID, sponsorshipID)
	if err != nil {
		return fmt.Errorf("unlinking sponsorship %d from episode %d: %w", sponsorshipID, episodeID, err)
	}
	return nil
}

// SponsorshipIDsForEpisode returns just the IDs for checkbox pre-selection.
func (s *SponsorshipStore) SponsorshipIDsForEpisode(episodeID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT sponsorship_id FROM episode_sponsorships WHERE episode_id = ?`, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing sponsorship IDs for episode %d: %w", episodeID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning sponsorship ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
