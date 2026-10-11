-- A poll can be closed before its end time by its creator or someone who
-- manages the channel; a closed poll takes no more votes and shows its results
-- to everyone who reads the channel.
ALTER TABLE polls ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;
ALTER TABLE polls ADD COLUMN IF NOT EXISTS closed_by TEXT REFERENCES users(id) ON DELETE SET NULL;
