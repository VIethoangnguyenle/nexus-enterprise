-- 030_users_email_verification.sql: an email on an account is only a claim until
-- the owner has proved it.
--
-- Signup stores the address as typed, so anyone can register an address they do
-- not own. Anything that treats an address as identity (an invitation to a
-- workspace is the first) must therefore use only a verified one. The column is
-- set from a Google sign-in whose email_verified is true, and from a one-time
-- code delivered to the address by a real sender; the test-only fixed code
-- never sets it.
--
-- Two steps, so a failure of the second leaves the first in place:
--   1. the column (idempotent);
--   2. addresses become case-insensitively unique. If two accounts already hold
--      the same address in different case, this FAILS LOUDLY and lists them. It
--      does not merge or delete accounts: that is a decision for a person.
--
-- Idempotent: `make db-migrate` re-applies every migration on each run.

ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;

-- Accounts already linked to Google with the same address were verified by
-- Google when they signed in; keep that proof.
UPDATE users u
   SET email_verified_at = ui.created_at
  FROM user_identities ui
 WHERE ui.user_id = u.id AND ui.provider = 'google'
   AND u.email_verified_at IS NULL
   AND u.email IS NOT NULL AND lower(trim(ui.email)) = lower(trim(u.email));

DO $$
DECLARE
    dup RECORD;
    report TEXT := '';
BEGIN
    FOR dup IN
        SELECT lower(trim(email)) AS addr, count(*) AS n, string_agg(id, ', ' ORDER BY id) AS ids
          FROM users
         WHERE email IS NOT NULL AND trim(email) <> ''
         GROUP BY lower(trim(email))
        HAVING count(*) > 1
    LOOP
        report := report || format(E'\n  %s held by %s accounts: %s', dup.addr, dup.n, dup.ids);
    END LOOP;

    IF report <> '' THEN
        RAISE EXCEPTION 'users.email has case-variant duplicates; resolve them by hand (accounts are not merged automatically), then re-run db-migrate:%', report;
    END IF;

    -- No duplicates: store the normalised form, then make it unique.
    UPDATE users SET email = lower(trim(email)) WHERE email IS NOT NULL AND email <> lower(trim(email));
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email_lower ON users (lower(email));
