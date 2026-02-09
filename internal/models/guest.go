package models

import (
	"database/sql"
	"fmt"
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
		twitter, instagram, linkedin, mastodon, image, created_at, updated_at
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
			&g.Image, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning guest: %w", err)
		}
		guests = append(guests, g)
	}
	return guests, rows.Err()
}

func (s *GuestStore) Get(id int64) (*Guest, error) {
	var g Guest
	err := s.db.QueryRow(`SELECT id, name, email, bio, website, company, podcast,
		twitter, instagram, linkedin, mastodon, image, created_at, updated_at
		FROM guests WHERE id = ?`, id).Scan(
		&g.ID, &g.Name, &g.Email, &g.Bio, &g.Website,
		&g.Company, &g.Podcast, &g.Twitter, &g.Instagram, &g.LinkedIn, &g.Mastodon,
		&g.Image, &g.CreatedAt, &g.UpdatedAt,
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
		twitter, instagram, linkedin, mastodon, image) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.Name, g.Email, g.Bio, g.Website, g.Company, g.Podcast,
		g.Twitter, g.Instagram, g.LinkedIn, g.Mastodon, g.Image)
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
		image = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		g.Name, g.Email, g.Bio, g.Website, g.Company, g.Podcast,
		g.Twitter, g.Instagram, g.LinkedIn, g.Mastodon, g.Image, g.ID)
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

// ShowsForAllGuests returns a map of guest ID → list of show names the guest has appeared on.
func (s *GuestStore) ShowsForAllGuests() (map[int64][]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT eg.guest_id, s.name
		FROM episode_guests eg
		JOIN episodes e ON e.id = eg.episode_id
		JOIN shows s ON s.id = e.show_id
		ORDER BY s.name`)
	if err != nil {
		return nil, fmt.Errorf("listing shows for all guests: %w", err)
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

// EpisodesForGuest returns all episode links for a guest.
func (s *GuestStore) EpisodesForGuest(guestID int64) ([]EpisodeGuest, error) {
	rows, err := s.db.Query(`SELECT eg.episode_id, eg.guest_id, eg.role, e.title
		FROM episode_guests eg
		JOIN episodes e ON e.id = eg.episode_id
		WHERE eg.guest_id = ?
		ORDER BY e.created_at DESC`, guestID)
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

// GuestsForEpisode returns all guests linked to an episode.
func (s *GuestStore) GuestsForEpisode(episodeID int64) ([]EpisodeGuest, error) {
	rows, err := s.db.Query(`SELECT eg.episode_id, eg.guest_id, eg.role, g.name
		FROM episode_guests eg
		JOIN guests g ON g.id = eg.guest_id
		WHERE eg.episode_id = ?
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
