-- What a user wants pushed to their devices. A user without a row gets the
-- defaults: every kind on, no quiet hours. Quiet hours are minutes after
-- midnight in the user's time zone; a window that wraps midnight (22:00–07:00)
-- has start > end.
CREATE TABLE IF NOT EXISTS push_preferences (
    user_id       TEXT        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    direct        BOOLEAN     NOT NULL DEFAULT TRUE,
    mention       BOOLEAN     NOT NULL DEFAULT TRUE,
    thread_reply  BOOLEAN     NOT NULL DEFAULT TRUE,
    quiet_enabled BOOLEAN     NOT NULL DEFAULT FALSE,
    quiet_start   SMALLINT    NOT NULL DEFAULT 1320 CHECK (quiet_start BETWEEN 0 AND 1439),
    quiet_end     SMALLINT    NOT NULL DEFAULT 420  CHECK (quiet_end BETWEEN 0 AND 1439),
    time_zone     TEXT        NOT NULL DEFAULT 'Asia/Ho_Chi_Minh',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
