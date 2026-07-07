package models

import (
	"database/sql"
	"fmt"
)

// dbtx is the subset of *sql.DB / *sql.Tx used by the query helpers, so the same
// sync logic can run either standalone (on the pool) or inside a larger
// transaction.
type dbtx interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// withTx runs fn inside a transaction, committing on success and rolling back on
// error (or panic). The deferred rollback is a no-op once Commit has run.
func withTx(db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
