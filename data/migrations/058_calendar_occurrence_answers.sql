-- A guest's answer to one occurrence of a repeating event. The answer to the
-- whole series lives in calendar_attendees; a row here overrides it for the
-- occurrence whose place in the series is original_start. It is a separate table
-- (not an exception row) so answering one occurrence does not freeze that
-- occurrence against later edits of the series.

CREATE TABLE IF NOT EXISTS calendar_attendee_occurrences (
    event_id       TEXT        NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    user_node_id   TEXT        NOT NULL,
    original_start TIMESTAMPTZ NOT NULL,
    response       TEXT        NOT NULL CHECK (response IN ('accepted', 'declined', 'tentative')),
    comment        TEXT        NOT NULL DEFAULT '',
    responded_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, user_node_id, original_start)
);

CREATE INDEX IF NOT EXISTS idx_calendar_attendee_occurrences_user
    ON calendar_attendee_occurrences (user_node_id);
