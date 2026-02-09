CREATE TABLE sponsorships (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    script TEXT NOT NULL DEFAULT '',
    cpm REAL,
    average_listens INTEGER,
    total_cost REAL,
    drop_date DATE,
    payment_due_date DATE,
    order_file TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE episode_sponsorships (
    episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    sponsorship_id INTEGER NOT NULL REFERENCES sponsorships(id) ON DELETE CASCADE,
    PRIMARY KEY (episode_id, sponsorship_id)
);
