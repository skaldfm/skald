UPDATE users SET role = 'user' WHERE role = 'editor';
UPDATE users SET role = 'user' WHERE role = 'viewer';
DROP TABLE user_shows;
