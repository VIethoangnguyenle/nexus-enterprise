-- 062_tenant_users_removed_status.sql
--
-- A person taken out of a workspace keeps their `tenant_users` row as a tombstone
-- (`status = 'removed'`) instead of the row being deleted. This file only widens the
-- CHECK constraint so the status is allowed; nothing is rewritten. The data
-- reconciliation that goes with it is 063.
--
-- Release train: this file ships before, or with, the code that writes 'removed'. The code
-- before it never writes that status, so applying it first changes nothing for it.
--
-- Re-runnable: the constraint is dropped if present and added again; the second run
-- changes nothing a reader can see. The new set is a superset of the old one, so rolling
-- back needs no reverse as long as no row is 'removed'; with rows 'removed' put them
-- back first (063 keeps the previous state of what it changed in _reconcile_063_tenant_users).

BEGIN;
ALTER TABLE tenant_users DROP CONSTRAINT IF EXISTS tenant_users_status_check;
ALTER TABLE tenant_users ADD CONSTRAINT tenant_users_status_check
    CHECK (status IN ('active', 'invited', 'disabled', 'removed'));
COMMIT;
