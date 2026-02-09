-- SQLite doesn't support DROP COLUMN before 3.35.0, so recreate table
CREATE TABLE guests_backup AS SELECT id, name, email, bio, website, created_at, updated_at FROM guests;
DROP TABLE guests;
CREATE TABLE guests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    website TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO guests SELECT * FROM guests_backup;
DROP TABLE guests_backup;
