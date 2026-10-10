-- Department approval steps name the department's NGAC user attribute, not its row id.
--
-- Group approval resolves who may act by checking whether the caller's user node reaches the
-- step's UA. Templates saved before that change could store departments.id as approver_value;
-- a request from such a template gets a group row no user can ever reach. New saves already
-- normalise to departments.ngac_ua_id; this rewrites the old steps in every tenant schema.
--
-- Department ids are globally unique, so matching on the id alone cannot pick another tenant's
-- department. Idempotent: a step that already holds a UA id matches no department row.

DO $$
DECLARE
    s TEXT;
BEGIN
    FOR s IN
        SELECT n.nspname FROM pg_namespace n
        WHERE n.nspname LIKE 'tenant\_%'
          AND to_regclass(quote_ident(n.nspname) || '.approval_steps') IS NOT NULL
    LOOP
        EXECUTE format(
            'UPDATE %I.approval_steps st
                SET approver_value = d.ngac_ua_id
               FROM departments d
              WHERE st.approver_type = %L
                AND st.approver_value = d.id',
            s, 'department');
    END LOOP;
END $$;
