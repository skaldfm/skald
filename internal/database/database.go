package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(dbURL, dataDir string) (*sql.DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	// Pragmas must be applied to every pooled connection, not just one, or
	// connections opened later run with foreign_keys off and no busy timeout.
	// modernc.org/sqlite applies DSN _pragma directives per-connection.
	sep := "?"
	if strings.Contains(dbURL, "?") {
		sep = "&"
	}
	dsn := dbURL + sep + "_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite allows only one writer; serialize access to avoid SQLITE_BUSY
	// errors under concurrent requests.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}
