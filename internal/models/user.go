package models

import (
	"database/sql"
	"fmt"
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
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
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

func (s *UserStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}
