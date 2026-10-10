-- 032_users_profile_completed.sql: remember whether a person has been asked who
-- they are.
--
-- A new account, whether it came from a one-time code or from Google, gets a
-- display name derived from its address ("hoa.le.novapay"). The sign-in flow
-- asks the person once for the name colleagues will see; this column is what
-- tells the client whether that step is still owed, so it does not depend on
-- which route the account came in by.
--
-- It is set the first time the profile is saved. Accounts that exist when the
-- column is added are marked done: they were never asked, and asking them now
-- would only interrupt people who are already working.
--
-- Idempotent: `make db-migrate` re-applies every migration on each run, so the
-- backfill happens only in the run that adds the column. An account created
-- afterwards keeps NULL until its owner completes the profile.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema() AND table_name = 'users'
           AND column_name = 'profile_completed_at'
    ) THEN
        ALTER TABLE users ADD COLUMN profile_completed_at TIMESTAMPTZ;
        UPDATE users SET profile_completed_at = COALESCE(created_at, NOW());
    END IF;
END $$;
