-- 021_approval_assignments_request_user_index.sql
--
-- Reading an approval request's audit trail checks whether the caller has any
-- assignment on that request: WHERE request_id = $1 AND user_node_id = $2.
-- Only (user_node_id, status) and (grant_source, status) were indexed.
--
-- provision_tenant_schema() (007) now creates this index for new tenants; this
-- adds it to every tenant schema that already exists. Idempotent.

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT schema_name
        FROM tenant_schemas
        WHERE schema_name IN (SELECT nspname FROM pg_namespace)
    LOOP
        IF to_regclass(format('%I.approval_assignments', rec.schema_name)) IS NOT NULL THEN
            EXECUTE format(
                'CREATE INDEX IF NOT EXISTS idx_aa_request_user ON %I.approval_assignments(request_id, user_node_id)',
                rec.schema_name);
        END IF;
    END LOOP;
END $$;
