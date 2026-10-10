-- 026_approval_assignments_one_action_per_person.sql
--
-- A role or department approver is stored as ONE group row on the step (its
-- user_node_id is the role or department UA). The first time a member acts, the
-- service inserts a row of their own (same grant source, approved/rejected), so
-- the step's quorum counts people. This unique index is what keeps one person
-- from acting twice on the same step.
--
-- provision_tenant_schema() (007) now creates the index for new tenants; this
-- adds it to every tenant schema that already exists. Rows that already share a
-- (request, step, user) are collapsed first, keeping the one that records an
-- action. Idempotent.

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
            EXECUTE format($q$
                DELETE FROM %1$I.approval_assignments a
                USING (
                    SELECT id, ROW_NUMBER() OVER (
                        PARTITION BY request_id, step_order, user_node_id
                        ORDER BY (status IN ('approved', 'rejected')) DESC, id
                    ) AS n
                    FROM %1$I.approval_assignments
                ) d
                WHERE a.id = d.id AND d.n > 1
            $q$, rec.schema_name);
            EXECUTE format(
                'CREATE UNIQUE INDEX IF NOT EXISTS uq_aa_request_step_user ON %I.approval_assignments(request_id, step_order, user_node_id)',
                rec.schema_name);
        END IF;
    END LOOP;
END $$;
