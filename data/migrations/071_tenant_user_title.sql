-- 071_tenant_user_title.sql
--
-- Chức danh belongs to the workspace (docs/specs/contacts-documents-settings-screens): in an organization it
-- is set on Hồ sơ trong tổ chức, so it is that organization's, and editing it in one workspace must not
-- change it in another.
--   tenant_users.title   the person's title in this workspace. NULL means "not set here": the account's
--                        default, users.title, is shown. '' is a title cleared in this workspace and shows empty.
--   users.title          stays as the account's default, written when no workspace is in scope (the profile
--                        step of sign-up, before any workspace is open).
--
-- Backfill: every existing membership keeps the title it showed before, copied from the account, so the
-- first edit in one workspace leaves the others as they were. Only rows still NULL are filled.
--
-- Re-runnable: IF NOT EXISTS and the NULL guard make `make db-migrate`, which re-applies every file, harmless.
--
-- Reverse (additive; the release before this one never reads it):
--   ALTER TABLE tenant_users DROP COLUMN IF EXISTS title;

BEGIN;

ALTER TABLE tenant_users ADD COLUMN IF NOT EXISTS title TEXT;

UPDATE tenant_users tu
   SET title = u.title
  FROM users u
 WHERE u.id = tu.user_id
   AND tu.title IS NULL
   AND COALESCE(u.title, '') <> '';

COMMIT;
