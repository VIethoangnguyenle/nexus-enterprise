-- Whether a person's devices are told of an incoming call (and of the call they
-- missed). On by default, like every other kind.
ALTER TABLE push_preferences ADD COLUMN IF NOT EXISTS incoming_call BOOLEAN NOT NULL DEFAULT TRUE;
