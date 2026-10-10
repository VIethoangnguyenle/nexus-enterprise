-- 036_invitation_last_emailed_at.sql: when an invitation was last emailed.
--
-- Inviting an address emails it. Inviting the same pending address again may
-- email it again, but not within a few minutes of the last one: the service
-- claims the send with one conditional update on this column, so two invites at
-- once send one message. NULL means never emailed (or a failed send gave the
-- slot back).

ALTER TABLE workspace_invitations
    ADD COLUMN IF NOT EXISTS last_emailed_at TIMESTAMPTZ;
