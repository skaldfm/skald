-- The previous unique index (migration 003) indexed season_number directly.
-- SQLite treats NULLs as distinct in unique indexes, so episodes with no season
-- (season_number IS NULL) bypassed the constraint entirely — (show, NULL, 5)
-- could be inserted any number of times. Collapse NULL season to -1 via an
-- expression index so those rows are deduplicated too.
--
-- NOTE: this CREATE fails if duplicate (show_id, season, episode_number) rows
-- already exist with a NULL season. The pre-migration backup covers rollback;
-- resolve duplicates manually and re-run if it fails.
DROP INDEX IF EXISTS idx_episodes_unique_number;
CREATE UNIQUE INDEX idx_episodes_unique_number
    ON episodes(show_id, COALESCE(season_number, -1), episode_number)
    WHERE episode_number IS NOT NULL;
