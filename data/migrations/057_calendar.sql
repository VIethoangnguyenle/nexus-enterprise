-- Calendar: personal, workspace and group calendars, their events, guests and reminders.
-- Authorization is NGAC, not these tables: a calendar is authorized on the OA in
-- ngac_oa_id, an event on that OA (or on its own OA once it has guests or a call).
-- Rows here are content and display caches; calendar_shares in particular only
-- mirrors the associations so the screen can list them.
-- Ids are TEXT like every other table; every row carries its workspace_id.

CREATE TABLE IF NOT EXISTS calendars (
    id            TEXT        PRIMARY KEY,
    workspace_id  TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    kind          TEXT        NOT NULL CHECK (kind IN ('personal', 'workspace', 'channel')),
    -- The owner's U node (personal calendars only).
    owner_node_id TEXT,
    channel_id    TEXT        REFERENCES channels(id) ON DELETE CASCADE,
    name          TEXT        NOT NULL DEFAULT '',
    color         SMALLINT    NOT NULL DEFAULT 0,
    ngac_oa_id    TEXT        NOT NULL REFERENCES ngac_nodes(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((kind = 'personal') = (owner_node_id IS NOT NULL)),
    CHECK ((kind = 'channel') = (channel_id IS NOT NULL))
);

-- One calendar of each kind per owner: lazy creation that races ends in a unique
-- violation, and the loser reads the winner's row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_calendars_personal
    ON calendars (workspace_id, owner_node_id) WHERE kind = 'personal';
CREATE UNIQUE INDEX IF NOT EXISTS idx_calendars_workspace
    ON calendars (workspace_id) WHERE kind = 'workspace';
CREATE UNIQUE INDEX IF NOT EXISTS idx_calendars_channel
    ON calendars (channel_id) WHERE kind = 'channel';

-- Which calendars a person has on their list, with their own colour and visibility.
CREATE TABLE IF NOT EXISTS calendar_subscriptions (
    user_node_id TEXT     NOT NULL,
    calendar_id  TEXT     NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    color        SMALLINT,
    visible      BOOLEAN  NOT NULL DEFAULT TRUE,
    position     INTEGER  NOT NULL DEFAULT 0,
    PRIMARY KEY (user_node_id, calendar_id)
);

-- Display cache of the shares of a personal calendar; the real grant is the
-- association from the grantee's personal UA to the calendar's OA.
CREATE TABLE IF NOT EXISTS calendar_shares (
    calendar_id     TEXT        NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    grantee_node_id TEXT        NOT NULL,
    level           TEXT        NOT NULL CHECK (level IN ('view', 'edit', 'manage')),
    granted_by      TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (calendar_id, grantee_node_id)
);

CREATE TABLE IF NOT EXISTS calendar_events (
    id                 TEXT        PRIMARY KEY,
    workspace_id       TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    calendar_id        TEXT        NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    organizer_node_id  TEXT        NOT NULL,
    title              TEXT        NOT NULL DEFAULT '',
    description        TEXT        NOT NULL DEFAULT '',
    location           TEXT        NOT NULL DEFAULT '',
    starts_at          TIMESTAMPTZ NOT NULL,
    ends_at            TIMESTAMPTZ NOT NULL,
    all_day            BOOLEAN     NOT NULL DEFAULT FALSE,
    -- The zone the start was chosen in; a repeating event expands in it.
    tzid               TEXT        NOT NULL DEFAULT 'UTC',
    rrule              TEXT,
    recurrence_until   TIMESTAMPTZ,
    -- An exception row points at its series and at the occurrence it replaces.
    recurring_event_id TEXT        REFERENCES calendar_events(id) ON DELETE CASCADE,
    original_start     TIMESTAMPTZ,
    cancelled          BOOLEAN     NOT NULL DEFAULT FALSE,
    kind               TEXT        NOT NULL DEFAULT 'event' CHECK (kind IN ('event', 'focus', 'ooo')),
    transparency       TEXT        NOT NULL DEFAULT 'busy' CHECK (transparency IN ('busy', 'free')),
    visibility         TEXT        NOT NULL DEFAULT 'default' CHECK (visibility IN ('default', 'private')),
    guests_can_invite  BOOLEAN     NOT NULL DEFAULT FALSE,
    guests_can_see_list BOOLEAN    NOT NULL DEFAULT TRUE,
    has_call           BOOLEAN     NOT NULL DEFAULT FALSE,
    -- Present only once the event has guests or a call.
    ngac_oa_id         TEXT        REFERENCES ngac_nodes(id) ON DELETE SET NULL,
    guests_ua_id       TEXT        REFERENCES ngac_nodes(id) ON DELETE SET NULL,
    version            INTEGER     NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at >= starts_at),
    CONSTRAINT calendar_events_exception_unique UNIQUE (recurring_event_id, original_start)
);

CREATE INDEX IF NOT EXISTS idx_calendar_events_range ON calendar_events (calendar_id, starts_at);
CREATE INDEX IF NOT EXISTS idx_calendar_events_series ON calendar_events (recurring_event_id)
    WHERE recurring_event_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS calendar_attendees (
    event_id     TEXT        NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    user_node_id TEXT        NOT NULL,
    response     TEXT        NOT NULL DEFAULT 'needs_action'
                 CHECK (response IN ('needs_action', 'accepted', 'declined', 'tentative')),
    comment      TEXT        NOT NULL DEFAULT '',
    is_organizer BOOLEAN     NOT NULL DEFAULT FALSE,
    responded_at TIMESTAMPTZ,
    PRIMARY KEY (event_id, user_node_id)
);

CREATE INDEX IF NOT EXISTS idx_calendar_attendees_user ON calendar_attendees (user_node_id);

-- user_node_id NULL is the event's default reminder; a person's own override has it set.
CREATE TABLE IF NOT EXISTS calendar_reminders (
    event_id       TEXT    NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    user_node_id   TEXT,
    minutes_before INTEGER NOT NULL CHECK (minutes_before >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_calendar_reminders_unique
    ON calendar_reminders (event_id, COALESCE(user_node_id, ''), minutes_before);

-- A reminder fires once per occurrence and person.
CREATE TABLE IF NOT EXISTS calendar_reminder_deliveries (
    event_id         TEXT        NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    user_node_id     TEXT        NOT NULL,
    occurrence_start TIMESTAMPTZ NOT NULL,
    minutes_before   INTEGER     NOT NULL,
    delivered_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, user_node_id, occurrence_start, minutes_before)
);

CREATE TABLE IF NOT EXISTS calendar_user_settings (
    user_node_id     TEXT NOT NULL,
    workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    tzid             TEXT NOT NULL DEFAULT 'Asia/Ho_Chi_Minh',
    work_start       TIME NOT NULL DEFAULT '09:00',
    work_end         TIME NOT NULL DEFAULT '18:00',
    default_reminder INTEGER NOT NULL DEFAULT 10,
    PRIMARY KEY (user_node_id, workspace_id)
);

-- A call belongs to a conversation or to a calendar event, never both. Existing
-- rows all have a channel, so the new check holds for them; the old behaviour
-- is unchanged for services that only know channel calls.
ALTER TABLE calls ALTER COLUMN channel_id DROP NOT NULL;
ALTER TABLE calls ADD COLUMN IF NOT EXISTS calendar_event_id TEXT
    REFERENCES calendar_events(id) ON DELETE CASCADE;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'calls_one_owner') THEN
        ALTER TABLE calls ADD CONSTRAINT calls_one_owner
            CHECK ((channel_id IS NULL) <> (calendar_event_id IS NULL));
    END IF;
END $$;

-- One open call per event, as for a conversation.
CREATE UNIQUE INDEX IF NOT EXISTS idx_calls_one_open_per_event
    ON calls (calendar_event_id) WHERE status IN ('ringing', 'active') AND calendar_event_id IS NOT NULL;

-- The graph keeps one node per platform name. Calendar, event and guest-group
-- nodes are named by ID, so a second node of the same name can only come from
-- two requests racing to create the same thing; the index makes the loser fail
-- and look the winner's node up instead of leaving an orphan.
CREATE UNIQUE INDEX IF NOT EXISTS ngac_nodes_calendar_name_uniq
    ON ngac_nodes (name, node_type) WHERE name ~ '^(Cal_|CalEv_|CalEvGuests_)';
