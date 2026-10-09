-- Prohibitions: deny overrides that block access an association would otherwise allow.
--
-- The table was defined only in backend/services/policy/migrations/, which nothing applies,
-- so a database built from init.sql and this chain lacked it. The policy service loads
-- prohibitions into its in-memory graph at startup and cannot start without the table.

CREATE TABLE IF NOT EXISTS ngac_prohibitions (
    id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name          TEXT UNIQUE NOT NULL,
    subject_id    TEXT NOT NULL,       -- U or UA node being denied
    operations    TEXT[] NOT NULL,     -- {"read","write"}
    target_oa_ids TEXT[] NOT NULL,     -- {OA_1, OA_2}
    intersection  BOOLEAN DEFAULT FALSE,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prohibitions_subject ON ngac_prohibitions(subject_id);
