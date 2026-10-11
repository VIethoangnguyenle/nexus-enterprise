-- A personal workspace can become an organization, once and for good. The two
-- columns record who did it and when; both stay empty for a workspace that was
-- made an organization, and converted_by is cleared if that person's account
-- is ever deleted (the time stays).
ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS converted_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS converted_at TIMESTAMPTZ;
