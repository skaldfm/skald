-- Shows
CREATE TABLE shows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    artwork TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Episodes
CREATE TABLE episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    episode_number INTEGER,
    season_number INTEGER,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'idea' CHECK(status IN ('idea', 'research', 'scripted', 'recorded', 'edited', 'published')),
    publish_date DATE,
    script TEXT NOT NULL DEFAULT '',
    show_notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_episodes_show_id ON episodes(show_id);
CREATE INDEX idx_episodes_status ON episodes(status);

-- Guests
CREATE TABLE guests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    website TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Episode-Guest linking
CREATE TABLE episode_guests (
    episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    guest_id INTEGER NOT NULL REFERENCES guests(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'guest',
    PRIMARY KEY (episode_id, guest_id)
);

-- Assets
CREATE TABLE assets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    filepath TEXT NOT NULL,
    filetype TEXT NOT NULL DEFAULT '',
    filesize INTEGER NOT NULL DEFAULT 0,
    asset_type TEXT NOT NULL DEFAULT 'other' CHECK(asset_type IN ('script', 'audio', 'artwork', 'notes', 'other')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_assets_episode_id ON assets(episode_id);

-- Tags
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

-- Episode-Tag linking
CREATE TABLE episode_tags (
    episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (episode_id, tag_id)
);
