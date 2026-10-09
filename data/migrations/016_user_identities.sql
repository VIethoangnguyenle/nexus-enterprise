-- 016_user_identities.sql
--
-- External sign-in identities (Sign in with Google).
--
-- A Google account is identified by its `sub` claim, which Google never
-- reassigns. Email is NOT an identifier: a Workspace admin can delete an
-- account and give its address to someone else. `email` here is recorded for
-- audit only and is never used to look a user up.
--
-- Also makes workspaces.domain unique (case-insensitively). Google sign-in
-- creates a tenant for a new hosted domain and claims the domain for it; two
-- tenants holding the same domain would make auto-join ambiguous.
--
-- Idempotent: `make db-migrate` re-applies every migration on each run.

BEGIN;

CREATE TABLE IF NOT EXISTS user_identities (
    provider      TEXT        NOT NULL CHECK (provider <> ''),
    subject       TEXT        NOT NULL CHECK (subject <> ''),
    user_id       TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email         TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, subject)
);

-- One identity per provider per user: a second Google account cannot be
-- attached to an account that already has one.
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_identities_user_provider
    ON user_identities (user_id, provider);

-- An empty string is "no domain", not a domain every such tenant shares; left
-- as '' it would trip the unique index below.
UPDATE workspaces SET domain = NULL WHERE btrim(domain) = '';

-- Only create the unique index if existing data allows it; otherwise report
-- the duplicates instead of aborting the whole migration chain.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM workspaces
        WHERE domain IS NOT NULL
        GROUP BY lower(domain) HAVING count(*) > 1
    ) THEN
        RAISE WARNING 'workspaces.domain has duplicates; idx_workspaces_domain_unique not created. Resolve them and re-run.';
    ELSE
        CREATE UNIQUE INDEX IF NOT EXISTS idx_workspaces_domain_unique
            ON workspaces (lower(domain)) WHERE domain IS NOT NULL;
    END IF;
END
$$;

COMMIT;
