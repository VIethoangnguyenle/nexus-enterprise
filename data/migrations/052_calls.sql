-- Voice and video calls inside a conversation. A call belongs to its channel and
-- is authorized on the channel's content OA like every message: a call is not a
-- graph node. The media itself runs in LiveKit; these rows hold the lifecycle
-- (who started it, ringing, answered, ended) and who took part, for the card in
-- the stream and the conversation previews.
CREATE TABLE IF NOT EXISTS calls (
    id              TEXT        PRIMARY KEY,
    channel_id      TEXT        NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    kind            TEXT        NOT NULL CHECK (kind IN ('audio', 'video')),
    status          TEXT        NOT NULL CHECK (status IN ('ringing', 'active', 'ended', 'missed', 'declined', 'cancelled')),
    started_by      TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- The card announcing the call in the conversation.
    message_id      TEXT        REFERENCES messages(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- When someone other than the caller first joined (a group call starts at once).
    started_at      TIMESTAMPTZ,
    ended_at        TIMESTAMPTZ,
    -- A direct call rings until this moment, then counts as missed.
    ring_expires_at TIMESTAMPTZ
);

-- One open call per conversation: starting a call while one rings or runs joins it.
CREATE UNIQUE INDEX IF NOT EXISTS idx_calls_one_open_per_channel
    ON calls (channel_id) WHERE status IN ('ringing', 'active');

-- The ring sweeper looks only at calls still ringing.
CREATE INDEX IF NOT EXISTS idx_calls_ringing_expiry
    ON calls (ring_expires_at) WHERE status = 'ringing';

-- Each stay of a person in a call; rejoining adds a row.
CREATE TABLE IF NOT EXISTS call_participants (
    call_id   TEXT        NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    user_id   TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at   TIMESTAMPTZ,
    PRIMARY KEY (call_id, user_id, joined_at)
);

CREATE INDEX IF NOT EXISTS idx_call_participants_present
    ON call_participants (user_id) WHERE left_at IS NULL;

-- The NGAC user node each person was checked as when they were last handed a
-- ticket into the call. A person has one node per workspace, so the webhook and
-- the sweep of people who lost access check that node again — never one looked
-- up from the user alone. Someone in the room with no row here was never given
-- a ticket by this service and is put out.
CREATE TABLE IF NOT EXISTS call_access (
    call_id      TEXT        NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    user_id      TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ngac_node_id TEXT        NOT NULL,
    -- The workspace the ticket was asked for in; the re-check runs as the
    -- person in that workspace.
    tenant_id    TEXT        NOT NULL DEFAULT '',
    granted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (call_id, user_id)
);
