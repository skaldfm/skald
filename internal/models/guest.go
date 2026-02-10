package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Guest struct {
	ID        int64
	Name      string
	Email     string
	Bio       string
	Website   string
	Company   string
	Podcast   string
	Twitter   string
	Instagram string
	LinkedIn  string
	Mastodon  string
	Image     string
	IsHost    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type EpisodeGuest struct {
	EpisodeID    int64
	GuestID      int64
	Role         string
	GuestName    string
	EpisodeTitle string
}

type GuestStore struct {
	db *sql.DB
}

func NewGuestStore(db *sql.DB) *GuestStore {
	return &GuestStore{db: db}
}

func (s *GuestStore) List() ([]Guest, error) {
	rows, err := s.db.Query(`SELECT id, name, email, bio, website, company, podcast,
		twitter, instagram, linkedin, mastodon, image, is_host, created_at, updated_at
		FROM guests ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing guests: %w", err)
	}
	defer rows.Close()

	var guests []Guest
	for rows.Next() {
		var g Guest
		if err := rows.Scan(&g.ID, &g.Name, &g.Email, &g.Bio, &g.Website,
			&g.Company, &g.Podcast, &g.Twitter, &g.Instagram, &g.LinkedIn, &g.Mastodon,
			&g.Image, &g.IsHost, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning guest: %w", err)
		}
		guests = append(guests, g)
	}
	return guests, rows.Err()
}

func (s *GuestStore) ListHosts() ([]Guest, error) {
	rows, err := s.db.Query(`SELECT id, name, email, bio, website, company, podcast,
		twitter, instagram, linkedin, mastodon, image, is_host, created_at, updated_at
		FROM guests WHERE is_host = 1 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}
	defer rows.Close()

	var guests []Guest
	for rows.Next() {
		var g Guest
		if err := rows.Scan(&g.ID, &g.Name, &g.Email, &g.Bio, &g.Website,
			&g.Company, &g.Podcast, &g.Twitter, &g.Instagram, &g.LinkedIn, &g.Mastodon,
			&g.Image, &g.IsHost, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning host: %w", err)
		}
		guests = append(guests, g)
	}
	return guests, rows.Err()
}

// ListByShowIDs returns guests linked to episodes in the given shows
// (via episode_guests) or designated as show hosts (via show_hosts).
func (s *GuestStore) ListByShowIDs(showIDs []int64) ([]Guest, error) {
	if len(showIDs) == 0 {
		return nil, nil
	}
	placeholders := "?" + strings.Repeat(",?", len(showIDs)-1)
	query := fmt.Sprintf(`SELECT DISTINCT g.id, g.name, g.email, g.bio, g.website, g.company, g.podcast,
		g.twitter, g.instagram, g.linkedin, g.mastodon, g.image, g.is_host, g.created_at, g.updated_at
		FROM guests g
		WHERE g.id IN (
			SELECT eg.guest_id FROM episode_guests eg
			JOIN episodes e ON e.id = eg.episode_id
			WHERE e.show_id IN (%s)
			UNION
			SELECT sh.guest_id FROM show_hosts sh
			WHERE sh.show_id IN (%s)
		)
		ORDER BY g.name`, placeholders, placeholders)

	args := make([]any, 0, len(showIDs)*2)
	for _, id := range showIDs {
		args = append(args, id)
	}
	for _, id := range showIDs {
		args = append(args, id)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing guests by show IDs: %w", err)
	}
	defer rows.Close()

	var guests []Guest
	for rows.Next() {
		var g Guest
		if err := rows.Scan(&g.ID, &g.Name, &g.Email, &g.Bio, &g.Website,
			&g.Company, &g.Podcast, &g.Twitter, &g.Instagram, &g.LinkedIn, &g.Mastodon,
			&g.Image, &g.IsHost, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning guest: %w", err)
		}
		guests = append(guests, g)
	}
	return guests, rows.Err()
}

func (s *GuestStore) Get(id int64) (*Guest, error) {
	var g Guest
	err := s.db.QueryRow(`SELECT id, name, email, bio, website, company, podcast,
		twitter, instagram, linkedin, mastodon, image, is_host, created_at, updated_at
		FROM guests WHERE id = ?`, id).Scan(
		&g.ID, &g.Name, &g.Email, &g.Bio, &g.Website,
		&g.Company, &g.Podcast, &g.Twitter, &g.Instagram, &g.LinkedIn, &g.Mastodon,
		&g.Image, &g.IsHost, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting guest %d: %w", id, err)
	}
	return &g, nil
}

func (s *GuestStore) Create(g *Guest) (*Guest, error) {
	result, err := s.db.Exec(`INSERT INTO guests (name, email, bio, website, company, podcast,
		twitter, instagram, linkedin, mastodon, image, is_host) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.Name, g.Email, g.Bio, g.Website, g.Company, g.Podcast,
		g.Twitter, g.Instagram, g.LinkedIn, g.Mastodon, g.Image, g.IsHost)
	if err != nil {
		return nil, fmt.Errorf("creating guest: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return s.Get(id)
}

func (s *GuestStore) Update(g *Guest) error {
	_, err := s.db.Exec(`UPDATE guests SET name = ?, email = ?, bio = ?, website = ?,
		company = ?, podcast = ?, twitter = ?, instagram = ?, linkedin = ?, mastodon = ?,
		image = ?, is_host = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		g.Name, g.Email, g.Bio, g.Website, g.Company, g.Podcast,
		g.Twitter, g.Instagram, g.LinkedIn, g.Mastodon, g.Image, g.IsHost, g.ID)
	if err != nil {
		return fmt.Errorf("updating guest %d: %w", g.ID, err)
	}
	return nil
}

func (s *GuestStore) UpdateImage(id int64, image string) error {
	_, err := s.db.Exec(`UPDATE guests SET image = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, image, id)
	if err != nil {
		return fmt.Errorf("updating guest %d image: %w", id, err)
	}
	return nil
}

func (s *GuestStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM guests WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting guest %d: %w", id, err)
	}
	return nil
}

// ShowsForGuests returns a map of guest ID → list of show names.
// If showIDs is nil, returns all shows; otherwise scopes to the given shows.
func (s *GuestStore) ShowsForGuests(showIDs []int64) (map[int64][]string, error) {
	query := `SELECT DISTINCT eg.guest_id, s.name
		FROM episode_guests eg
		JOIN episodes e ON e.id = eg.episode_id
		JOIN shows s ON s.id = e.show_id`
	var args []any
	if showIDs != nil {
		if len(showIDs) == 0 {
			return make(map[int64][]string), nil
		}
		query += " WHERE e.show_id IN (?" + strings.Repeat(",?", len(showIDs)-1) + ")"
		for _, id := range showIDs {
			args = append(args, id)
		}
	}
	query += " ORDER BY s.name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing shows for guests: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]string)
	for rows.Next() {
		var guestID int64
		var showName string
		if err := rows.Scan(&guestID, &showName); err != nil {
			return nil, fmt.Errorf("scanning guest show: %w", err)
		}
		result[guestID] = append(result[guestID], showName)
	}
	return result, rows.Err()
}

// EpisodesForGuest returns episode links for a guest.
// If showIDs is nil, returns all; otherwise scopes to the given shows.
func (s *GuestStore) EpisodesForGuest(guestID int64, showIDs []int64) ([]EpisodeGuest, error) {
	query := `SELECT eg.episode_id, eg.guest_id, eg.role, e.title
		FROM episode_guests eg
		JOIN episodes e ON e.id = eg.episode_id
		WHERE eg.guest_id = ?`
	args := []any{guestID}
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
		return nil, fmt.Errorf("listing episodes for guest %d: %w", guestID, err)
	}
	defer rows.Close()

	var links []EpisodeGuest
	for rows.Next() {
		var eg EpisodeGuest
		if err := rows.Scan(&eg.EpisodeID, &eg.GuestID, &eg.Role, &eg.EpisodeTitle); err != nil {
			return nil, fmt.Errorf("scanning episode guest: %w", err)
		}
		links = append(links, eg)
	}
	return links, rows.Err()
}

// GuestsForEpisode returns non-host guests linked to an episode.
func (s *GuestStore) GuestsForEpisode(episodeID int64) ([]EpisodeGuest, error) {
	rows, err := s.db.Query(`SELECT eg.episode_id, eg.guest_id, eg.role, g.name
		FROM episode_guests eg
		JOIN guests g ON g.id = eg.guest_id
		WHERE eg.episode_id = ? AND eg.role != 'host'
		ORDER BY g.name`, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing guests for episode %d: %w", episodeID, err)
	}
	defer rows.Close()

	var links []EpisodeGuest
	for rows.Next() {
		var eg EpisodeGuest
		if err := rows.Scan(&eg.EpisodeID, &eg.GuestID, &eg.Role, &eg.GuestName); err != nil {
			return nil, fmt.Errorf("scanning episode guest: %w", err)
		}
		links = append(links, eg)
	}
	return links, rows.Err()
}

// GuestIDsForEpisode returns the IDs of non-host guests linked to an episode.
func (s *GuestStore) GuestIDsForEpisode(episodeID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT guest_id FROM episode_guests WHERE episode_id = ? AND role != 'host'`, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing guest IDs for episode %d: %w", episodeID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning guest ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// LinkGuest links a guest to an episode with a role.
func (s *GuestStore) LinkGuest(episodeID, guestID int64, role string) error {
	if role == "" {
		role = "guest"
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO episode_guests (episode_id, guest_id, role) VALUES (?, ?, ?)`,
		episodeID, guestID, role)
	if err != nil {
		return fmt.Errorf("linking guest %d to episode %d: %w", guestID, episodeID, err)
	}
	return nil
}

// UnlinkGuest removes a guest from an episode.
func (s *GuestStore) UnlinkGuest(episodeID, guestID int64) error {
	_, err := s.db.Exec(`DELETE FROM episode_guests WHERE episode_id = ? AND guest_id = ?`,
		episodeID, guestID)
	if err != nil {
		return fmt.Errorf("unlinking guest %d from episode %d: %w", guestID, episodeID, err)
	}
	return nil
}

// HostsForShow returns all guests designated as hosts for a show.
func (s *GuestStore) HostsForShow(showID int64) ([]Guest, error) {
	rows, err := s.db.Query(`SELECT g.id, g.name, g.email, g.bio, g.website, g.company, g.podcast,
		g.twitter, g.instagram, g.linkedin, g.mastodon, g.image, g.is_host, g.created_at, g.updated_at
		FROM guests g
		JOIN show_hosts sh ON sh.guest_id = g.id
		WHERE sh.show_id = ?
		ORDER BY g.name`, showID)
	if err != nil {
		return nil, fmt.Errorf("listing hosts for show %d: %w", showID, err)
	}
	defer rows.Close()

	var guests []Guest
	for rows.Next() {
		var g Guest
		if err := rows.Scan(&g.ID, &g.Name, &g.Email, &g.Bio, &g.Website,
			&g.Company, &g.Podcast, &g.Twitter, &g.Instagram, &g.LinkedIn, &g.Mastodon,
			&g.Image, &g.IsHost, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning host: %w", err)
		}
		guests = append(guests, g)
	}
	return guests, rows.Err()
}

// HostIDsForShow returns the guest IDs of hosts for a show.
func (s *GuestStore) HostIDsForShow(showID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT guest_id FROM show_hosts WHERE show_id = ?`, showID)
	if err != nil {
		return nil, fmt.Errorf("listing host IDs for show %d: %w", showID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning host ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetShowHosts replaces all hosts for a show with the given guest IDs.
func (s *GuestStore) SetShowHosts(showID int64, guestIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM show_hosts WHERE show_id = ?`, showID); err != nil {
		return fmt.Errorf("clearing show hosts: %w", err)
	}
	for _, gid := range guestIDs {
		if _, err := tx.Exec(`INSERT INTO show_hosts (show_id, guest_id) VALUES (?, ?)`, showID, gid); err != nil {
			return fmt.Errorf("inserting show host %d: %w", gid, err)
		}
	}
	return tx.Commit()
}

// HostsForEpisode returns episode_guests with role='host' for an episode.
func (s *GuestStore) HostsForEpisode(episodeID int64) ([]EpisodeGuest, error) {
	rows, err := s.db.Query(`SELECT eg.episode_id, eg.guest_id, eg.role, g.name
		FROM episode_guests eg
		JOIN guests g ON g.id = eg.guest_id
		WHERE eg.episode_id = ? AND eg.role = 'host'
		ORDER BY g.name`, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing hosts for episode %d: %w", episodeID, err)
	}
	defer rows.Close()

	var links []EpisodeGuest
	for rows.Next() {
		var eg EpisodeGuest
		if err := rows.Scan(&eg.EpisodeID, &eg.GuestID, &eg.Role, &eg.GuestName); err != nil {
			return nil, fmt.Errorf("scanning episode host: %w", err)
		}
		links = append(links, eg)
	}
	return links, rows.Err()
}

// HostIDsForEpisode returns the guest IDs of hosts for an episode.
func (s *GuestStore) HostIDsForEpisode(episodeID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT guest_id FROM episode_guests WHERE episode_id = ? AND role = 'host'`, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing host IDs for episode %d: %w", episodeID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning host ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
