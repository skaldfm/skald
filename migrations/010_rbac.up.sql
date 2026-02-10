CREATE TABLE user_shows (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, show_id)
);

UPDATE users SET role = 'editor' WHERE role = 'user';
