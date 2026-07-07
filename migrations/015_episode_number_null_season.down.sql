DROP INDEX IF EXISTS idx_episodes_unique_number;
CREATE UNIQUE INDEX idx_episodes_unique_number
    ON episodes(show_id, season_number, episode_number)
    WHERE episode_number IS NOT NULL;
