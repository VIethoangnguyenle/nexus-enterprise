-- A stay in a call is one connection, not one person: a second device can take
-- the first one's place and LiveKit may report its arrival before the first
-- one's departure. Each stay remembers the LiveKit participant (connection) it
-- was, so a departure closes that stay only.
ALTER TABLE call_participants ADD COLUMN IF NOT EXISTS participant_sid TEXT NOT NULL DEFAULT '';
