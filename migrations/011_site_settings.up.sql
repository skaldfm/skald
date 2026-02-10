CREATE TABLE site_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    logo_path TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO site_settings (id) VALUES (1);
