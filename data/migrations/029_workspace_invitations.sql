-- 029_workspace_invitations.sql: pending invitations to a workspace, by email.
--
-- An invitation is a standing offer, not a membership: nothing is assigned in
-- the graph until the invited person accepts. It is matched to the person by
-- the address on their account at the time they answer, so the invite endpoint
-- never has to say whether an account exists. A role and a department may ride
-- along; they are re-checked against the inviter's rights when it is accepted.

CREATE TABLE IF NOT EXISTS workspace_invitations (
    id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    -- Trimmed and lower-cased by the service.
    email         TEXT NOT NULL,
    role_id       TEXT REFERENCES ngac_nodes(id) ON DELETE SET NULL,
    department_id TEXT REFERENCES departments(id) ON DELETE SET NULL,
    -- The inviter's user node. No foreign key: the invitation outlives a node a
    -- removed inviter had, and is then refused at accept time.
    invited_by    TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'accepted', 'declined', 'revoked')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL,
    responded_at  TIMESTAMPTZ
);

-- One pending offer per address per workspace: inviting again refreshes it.
CREATE UNIQUE INDEX IF NOT EXISTS uq_workspace_invitations_pending
    ON workspace_invitations (workspace_id, email) WHERE status = 'pending';

-- "My invitations" looks up by address.
CREATE INDEX IF NOT EXISTS idx_workspace_invitations_email
    ON workspace_invitations (email) WHERE status = 'pending';
