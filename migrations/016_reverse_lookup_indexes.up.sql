-- Reverse-lookup indexes. The junction tables' composite primary keys
-- (episode_id, ...) index lookups by episode, but "which episodes reference this
-- guest/sponsor" scans the whole table. Add the reverse-direction indexes.
CREATE INDEX IF NOT EXISTS idx_episode_guests_guest_id ON episode_guests(guest_id);
CREATE INDEX IF NOT EXISTS idx_episode_sponsorships_sponsorship_id ON episode_sponsorships(sponsorship_id);

-- The default episode listing orders by updated_at DESC.
CREATE INDEX IF NOT EXISTS idx_episodes_updated_at ON episodes(updated_at);
