package models

import (
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"
)

type User struct {
	ID           int64
	Email        string
	DisplayName  string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserStore struct {
	db *sql.DB
	// hasUser caches "at least one account exists". It only ever flips false→true
	// (accounts aren't deleted to zero in normal use, and a restore restarts the
	// process), so once set we can skip the per-request COUNT for setup detection.
	hasUser atomic.Bool
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// HasAnyUser reports whether any account exists, used for the first-run setup
// redirect on every request. The positive result is cached to avoid a COUNT on
// the hot path.
func (s *UserStore) HasAnyUser() (bool, error) {
	if s.hasUser.Load() {
		return true, nil
	}
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking for users: %w", err)
	}
	if exists {
		s.hasUser.Store(true)
	}
	return exists, nil
}

func (s *UserStore) Get(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, email, display_name, password_hash, role, created_at, updated_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting user %d: %w", id, err)
	}
	return u, nil
}

func (s *UserStore) GetByEmail(email string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, email, display_name, password_hash, role, created_at, updated_at FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return u, nil
}

func (s *UserStore) Create(email, displayName, passwordHash, role string) (*User, error) {
	result, err := s.db.Exec(
		`INSERT INTO users (email, display_name, password_hash, role) VALUES (?, ?, ?, ?)`,
		email, displayName, passwordHash, role,
	)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	id, _ := result.LastInsertId()
	return s.Get(id)
}

func (s *UserStore) Update(u *User) error {
	_, err := s.db.Exec(
		`UPDATE users SET email = ?, display_name = ?, password_hash = ?, role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		u.Email, u.DisplayName, u.PasswordHash, u.Role, u.ID,
	)
	if err != nil {
		return fmt.Errorf("updating user %d: %w", u.ID, err)
	}
	return nil
}

func (s *UserStore) List() ([]*User, error) {
	rows, err := s.db.Query(
		`SELECT id, email, display_name, password_hash, role, created_at, updated_at FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *UserStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting user %d: %w", id, err)
	}
	return nil
}


func (s *UserStore) ShowIDsForUser(userID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT show_id FROM user_shows WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("getting show IDs for user %d: %w", userID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning show ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// AllUserShows returns every user's accessible show IDs in a single query,
// keyed by user ID. Avoids a per-user round trip on the admin users page.
func (s *UserStore) AllUserShows() (map[int64][]int64, error) {
	rows, err := s.db.Query(`SELECT user_id, show_id FROM user_shows`)
	if err != nil {
		return nil, fmt.Errorf("getting user shows: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]int64)
	for rows.Next() {
		var userID, showID int64
		if err := rows.Scan(&userID, &showID); err != nil {
			return nil, fmt.Errorf("scanning user show: %w", err)
		}
		result[userID] = append(result[userID], showID)
	}
	return result, rows.Err()
}

func (s *UserStore) SetUserShows(userID int64, showIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM user_shows WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("clearing user shows: %w", err)
	}
	for _, sid := range showIDs {
		if _, err := tx.Exec(`INSERT INTO user_shows (user_id, show_id) VALUES (?, ?)`, userID, sid); err != nil {
			return fmt.Errorf("inserting user show %d: %w", sid, err)
		}
	}
	return tx.Commit()
}
