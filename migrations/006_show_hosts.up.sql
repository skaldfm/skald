CREATE TABLE show_hosts (
    show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
    guest_id INTEGER NOT NULL REFERENCES guests(id) ON DELETE CASCADE,
    PRIMARY KEY (show_id, guest_id)
);
