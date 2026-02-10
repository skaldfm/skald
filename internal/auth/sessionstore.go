package auth

import (
	"database/sql"
	"time"
)

// SQLiteStore implements the scs.Store interface for modernc.org/sqlite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed session store.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// Find returns the data for a session token. If the token is not found or
// expired, the exists flag will be false.
func (s *SQLiteStore) Find(token string) ([]byte, bool, error) {
	var data []byte
	var expiry float64
	err := s.db.QueryRow(
		`SELECT data, expiry FROM sessions WHERE token = ?`, token,
	).Scan(&data, &expiry)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if time.Now().Unix() > int64(expiry) {
		return nil, false, nil
	}
	return data, true, nil
}

// Commit adds or updates a session token with the given data and expiry.
func (s *SQLiteStore) Commit(token string, data []byte, expiry time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, data, expiry) VALUES (?, ?, ?)
		 ON CONFLICT(token) DO UPDATE SET data = excluded.data, expiry = excluded.expiry`,
		token, data, float64(expiry.Unix()),
	)
	return err
}

// Delete removes a session token.
func (s *SQLiteStore) Delete(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// All returns a map of all active session tokens to their data.
func (s *SQLiteStore) All() (map[string][]byte, error) {
	rows, err := s.db.Query(`SELECT token, data FROM sessions WHERE expiry > ?`, float64(time.Now().Unix()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make(map[string][]byte)
	for rows.Next() {
		var token string
		var data []byte
		if err := rows.Scan(&token, &data); err != nil {
			return nil, err
		}
		sessions[token] = data
	}
	return sessions, rows.Err()
}

// Cleanup removes expired sessions. Called periodically by SCS.
func (s *SQLiteStore) Cleanup(interval time.Duration) (chan struct{}, error) {
	quit := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.db.Exec(`DELETE FROM sessions WHERE expiry < ?`, float64(time.Now().Unix())) //nolint:errcheck
			case <-quit:
				return
			}
		}
	}()
	return quit, nil
}
